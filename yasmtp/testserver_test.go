package yasmtp_test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yasmtp"
)

const (
	testCertValidityWindow = time.Hour
	testCertKeyBits        = 2048
	testCertSerialNumber   = 1
	testServerHost         = "127.0.0.1"
)

type capturedMail struct {
	From string
	To   []string
	Data string
}

type fakeSMTPServer struct {
	listener  net.Listener
	tlsConfig *tls.Config

	mu    sync.Mutex
	mails []capturedMail
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()

	cert := generateSelfSignedCert(t)

	listener, err := net.Listen("tcp", testServerHost+":0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := &fakeSMTPServer{
		listener:  listener,
		tlsConfig: &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
	}

	go server.serve()

	t.Cleanup(func() {
		_ = listener.Close()
	})

	return server
}

func (s *fakeSMTPServer) Host() yasmtp.Host {
	host, _, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		return yasmtp.Host(testServerHost)
	}

	return yasmtp.Host(host)
}

func (s *fakeSMTPServer) Port(t *testing.T) yasmtp.Port {
	t.Helper()

	_, port, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to split listener address: %v", err)
	}

	value, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		t.Fatalf("failed to parse listener port: %v", err)
	}

	return yasmtp.Port(value)
}

func (s *fakeSMTPServer) CertPool() *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(s.tlsConfig.Certificates[0].Leaf)

	return pool
}

func (s *fakeSMTPServer) Mails() []capturedMail {
	s.mu.Lock()
	defer s.mu.Unlock()

	mails := make([]capturedMail, len(s.mails))
	copy(mails, s.mails)

	return mails
}

func (s *fakeSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	writeSMTPLine(writer, "220 localhost ESMTP")

	var (
		inTLS bool
		mail  capturedMail
	)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(upper, "EHLO"):
			writeSMTPLine(writer, "250-localhost Hello")

			if inTLS {
				writeSMTPLine(writer, "250-AUTH PLAIN")
			} else {
				writeSMTPLine(writer, "250-STARTTLS")
			}

			writeSMTPLine(writer, "250 8BITMIME")
		case strings.HasPrefix(upper, "STARTTLS"):
			writeSMTPLine(writer, "220 Ready to start TLS")

			tlsConn := tls.Server(conn, s.tlsConfig)
			if handshakeErr := tlsConn.Handshake(); handshakeErr != nil {
				return
			}

			conn = tlsConn
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
			inTLS = true
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			writeSMTPLine(writer, "235 2.7.0 Authentication successful")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			mail = capturedMail{From: extractSMTPAddr(line)}
			writeSMTPLine(writer, "250 2.1.0 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			mail.To = append(mail.To, extractSMTPAddr(line))
			writeSMTPLine(writer, "250 2.1.5 OK")
		case strings.HasPrefix(upper, "DATA"):
			writeSMTPLine(writer, "354 Start mail input")
			mail.Data = s.readDataBlock(reader)

			s.mu.Lock()
			s.mails = append(s.mails, mail)
			s.mu.Unlock()

			writeSMTPLine(writer, "250 2.0.0 OK: queued")
		case strings.HasPrefix(upper, "RSET"):
			mail = capturedMail{}
			writeSMTPLine(writer, "250 2.0.0 OK")
		case strings.HasPrefix(upper, "NOOP"):
			writeSMTPLine(writer, "250 2.0.0 OK")
		case strings.HasPrefix(upper, "QUIT"):
			writeSMTPLine(writer, "221 2.0.0 Bye")

			return
		default:
			writeSMTPLine(writer, "500 5.5.1 Command not recognized")
		}
	}
}

func (s *fakeSMTPServer) readDataBlock(reader *bufio.Reader) string {
	var lines []string

	for {
		dataLine, err := reader.ReadString('\n')
		if err != nil {
			return strings.Join(lines, "\r\n")
		}

		trimmed := strings.TrimRight(dataLine, "\r\n")
		if trimmed == "." {
			break
		}

		lines = append(lines, trimmed)
	}

	return strings.Join(lines, "\r\n")
}

func writeSMTPLine(writer *bufio.Writer, line string) {
	_, _ = writer.WriteString(line)
	_, _ = writer.WriteString("\r\n")
	_ = writer.Flush()
}

func extractSMTPAddr(line string) string {
	start := strings.Index(line, "<")
	end := strings.Index(line, ">")

	if start == -1 || end == -1 || end <= start {
		return line
	}

	return line[start+1 : end]
}

func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, testCertKeyBits)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(testCertSerialNumber),
		Subject:      pkix.Name{CommonName: testServerHost},
		NotBefore:    time.Now().Add(-testCertValidityWindow),
		NotAfter:     time.Now().Add(testCertValidityWindow),
		IPAddresses:  []net.IP{net.ParseIP(testServerHost)},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        leaf,
	}
}
