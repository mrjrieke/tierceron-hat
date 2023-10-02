package cap

import (
	context "context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lafriks/go-shamir"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/sys/unix"
	grpc "google.golang.org/grpc"
)

const (
	FEATHER_COMMON = 1 << iota // COMMON
	FEATHER_CTL    = 1 << iota // CTL 2
	FEATHER_SECRET = 1 << iota // SECRET 4
)

const (
	MODE_FEATHER = "f"
	MODE_GLIDE   = "g"
)

var penseCodeMap map[string]string = map[string]string{}
var penseMemoryMap map[string]string = map[string]string{}

var penseFeatherCodeMap map[string]string = map[string]string{}
var penseFeatherMemoryMap map[string]string = map[string]string{}

var penseFeatherCtlCodeMap = cmap.New[string]()

const penseSocket = "./snap.sock"

func TapServer(address string, opt ...grpc.ServerOption) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var s *grpc.Server
	if opt != nil {
		s = grpc.NewServer(opt...)
	} else {
		s = grpc.NewServer()
	}
	RegisterCapServer(s, &penseServer{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

var clientCodeMap map[string][][]byte = map[string][][]byte{}

func handleMessage(handshakeCode string, conn *kcp.UDPSession, acceptRemote func(int, string) bool) {
	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := conn.Read(buf)
		if _, ok := clientCodeMap[conn.RemoteAddr().String()]; !ok {
			clientCodeMap[conn.RemoteAddr().String()] = [][]byte{}
		}

		if n == 0 || err != nil {
			// All done... hopefully.
			if _, ok := clientCodeMap[conn.RemoteAddr().String()]; ok {
				var messageBytes []byte
				var err error = nil
				if len(clientCodeMap[conn.RemoteAddr().String()]) > 1 {
					messageBytes, err = shamir.Combine(clientCodeMap[conn.RemoteAddr().String()]...)
				} else {
					if acceptRemote(FEATHER_CTL, conn.RemoteAddr().String()) {
						messageBytes = clientCodeMap[conn.RemoteAddr().String()][0]
						clientCodeMap[conn.RemoteAddr().String()][0] = []byte{}
						message := string(messageBytes)
						messageParts := strings.Split(message, ":")
						if messageParts[0] == handshakeCode {
							// handshake:featherctl:
							if messageParts[1] == "featherctl" && len(messageParts) == 4 {
								var msg string = ""
								var ok bool
								if msg, ok = penseFeatherCtlCodeMap.Get(messageParts[3]); !ok {
									// Default is Glide
									msg = MODE_GLIDE
								}
								switch messageParts[2] {
								case MODE_FEATHER: // Feather
									penseFeatherCtlCodeMap.Set(messageParts[3], MODE_FEATHER)
								case MODE_GLIDE: // Glide
									penseFeatherCtlCodeMap.Set(messageParts[3], MODE_GLIDE)
								}
								conn.Write([]byte(msg))
								defer conn.Close()
								return
							}
						}
					}
				}
				if err == nil {
					if acceptRemote(FEATHER_SECRET, conn.RemoteAddr().String()) {
						message := string(messageBytes)
						messageParts := strings.Split(message, ":")
						if messageParts[0] == handshakeCode {
							if len(messageParts[1]) == 64 {
								penseFeatherCodeMap[messageParts[1]] = ""
							}
						}
					}
				}
			}
			conn.Write([]byte(" "))
			defer conn.Close()
			return
		} else {
			clientCodeMap[conn.RemoteAddr().String()] = append(clientCodeMap[conn.RemoteAddr().String()], append([]byte{}, buf[:n]...))
		}
	}
}

func Feather(encryptPass string, encryptSalt string, port string, handshakeCode string, acceptRemote func(int, string) bool) {
	key := pbkdf2.Key([]byte(encryptPass), []byte(encryptSalt), 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)
	if listener, err := kcp.ListenWithOptions("127.0.0.1:"+port, block, 10, 3); err == nil {
		for {
			s, err := listener.AcceptKCP()
			if err != nil {
				log.Fatal(err)
			}
			if acceptRemote(FEATHER_COMMON, s.RemoteAddr().String()) {
				go handleMessage(handshakeCode, s, acceptRemote)
			} else {
				s.Close()
			}
		}
	}
}

func Tap(target string, expectedSha256 string) error {
	listener, err := net.Listen("unix", penseSocket)
	if err != nil {
		return err
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func(c chan os.Signal) {
		<-c
		listener.Close()
		os.Exit(0)
	}(signalChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if conn != nil {
				conn.Close()
			}
			return err
		}

		// 1st check.
		if conn.RemoteAddr().Network() == conn.LocalAddr().Network() {

			sysConn, sysConnErr := conn.(*net.UnixConn).SyscallConn()
			if sysConnErr != nil {
				conn.Close()
				continue
			}

			var cred *unix.Ucred
			var credErr error

			sysConn.Control(func(fd uintptr) {
				cred, credErr = unix.GetsockoptUcred(int(fd),
					unix.SOL_SOCKET,
					unix.SO_PEERCRED)
			})
			if credErr != nil {
				conn.Close()
				continue
			}

			path, linkErr := os.Readlink("/proc/" + strconv.Itoa(int(cred.Pid)) + "/exe")
			if linkErr != nil {
				conn.Close()
				continue
			}
			defer conn.Close()

			// 2nd check.
			if path == target {
				// 3rd check.
				peerExe, err := os.Open(path)
				if err != nil {
					conn.Close()
					continue
				}
				defer peerExe.Close()

				h := sha256.New()
				if _, err := io.Copy(h, peerExe); err != nil {
					conn.Close()
					continue
				}

				if expectedSha256 == hex.EncodeToString(h.Sum(nil)) {
					messageBytes := make([]byte, 64)

					err := sysConn.Read(func(s uintptr) bool {
						_, operr := syscall.Read(int(s), messageBytes)
						return operr != syscall.EAGAIN
					})
					if err != nil {
						conn.Close()
						continue
					}
					message := string(messageBytes)

					if len(message) == 64 {
						penseCodeMap[message] = ""
					}
				}

			}

		}
		conn.Close()
	}
}

func TapWriter(pense string) error {
	penseConn, penseErr := net.Dial("unix", penseSocket)
	if penseErr != nil {
		return penseErr
	}
	_, penseWriteErr := penseConn.Write([]byte(pense))
	defer penseConn.Close()
	if penseWriteErr != nil {
		return penseWriteErr
	}

	_, penseResponseErr := io.ReadAll(penseConn)

	return penseResponseErr
}

func FeatherCtlEmit(encryptPass string, encryptSalt string, hostAddr string, handshakeCode string, mode string, pense string) (string, error) {
	key := pbkdf2.Key([]byte(encryptPass), []byte(encryptSalt), 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)

	penseConn, penseErr := kcp.DialWithOptions(hostAddr, block, 10, 3)
	if penseErr != nil {
		return "", penseErr
	}
	defer penseConn.Close()
	_, penseWriteErr := penseConn.Write([]byte(handshakeCode + ":featherctl:" + mode + ":" + pense))
	if penseWriteErr != nil {
		return "", penseWriteErr
	}

	responseBuf := []byte{1}
	_, penseResponseErr := io.ReadFull(penseConn, responseBuf)

	return string(responseBuf), penseResponseErr
}

func FeatherWriter(encryptPass string, encryptSalt string, hostAddr string, handshakeCode string, pense string) ([]byte, error) {
	penseSplits, err := shamir.Split([]byte(handshakeCode+":"+pense), 12, 7)
	if err != nil {
		return nil, err
	}
	key := pbkdf2.Key([]byte(encryptPass), []byte(encryptSalt), 1024, 32, sha1.New)
	block, _ := kcp.NewAESBlockCrypt(key)

	penseConn, penseErr := kcp.DialWithOptions(hostAddr, block, 10, 3)
	if penseErr != nil {
		return nil, penseErr
	}
	defer penseConn.Close()
	for _, penseBlock := range penseSplits {
		_, penseWriteErr := penseConn.Write(penseBlock)
		if penseWriteErr != nil {
			return nil, penseWriteErr
		}
	}

	responseBuf := []byte{1}
	_, penseResponseErr := io.ReadFull(penseConn, responseBuf)

	return responseBuf, penseResponseErr
}

func TapFeather(penseIndex, memory string) {
	penseMemoryMap[penseIndex] = memory
	penseFeatherMemoryMap[penseIndex] = memory
}

func TapMemorize(penseIndex, memory string) {
	penseMemoryMap[penseIndex] = memory
}

type penseServer struct {
	UnimplementedCapServer
}

func (cs *penseServer) Pense(ctx context.Context, penseRequest *PenseRequest) (*PenseReply, error) {

	penseArray := sha256.Sum256([]byte(penseRequest.Pense))
	penseCode := hex.EncodeToString(penseArray[:])
	if _, penseCodeOk := penseCodeMap[penseCode]; penseCodeOk {
		delete(penseCodeMap, penseCode)

		if pense, penseOk := penseMemoryMap[penseRequest.PenseIndex]; penseOk {
			return &PenseReply{Pense: pense}, nil
		} else {
			return &PenseReply{Pense: "Pense undefined"}, nil
		}
	} else {
		// Might be a feather
		if _, penseCodeOk := penseFeatherCodeMap[penseCode]; penseCodeOk {
			delete(penseFeatherCodeMap, penseCode)
			if pense, penseOk := penseFeatherMemoryMap[penseRequest.PenseIndex]; penseOk {
				return &PenseReply{Pense: pense}, nil
			} else {
				return &PenseReply{Pense: "Pense undefined"}, nil
			}
		}
		return &PenseReply{Pense: "...."}, nil
	}
}

func main() {
	ex, err := os.Executable()
	if err != nil {
		os.Exit(-1)
	}
	exePath := filepath.Dir(ex)
	brimPath := strings.Replace(exePath, "/Cap", "/brim", 1)
	go Tap(brimPath, "f19431f322ea015ef871d267cc75e58b73d16617f9ff47ed7e0f0c1dbfb276b5")
	TapServer("127.0.0.1:1534")

}
