package smtpserver

import (
	"bytes"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/interaction"
)

type Server struct {
	cfg    *config.Config
	logger *interaction.Logger
}

func New(cfg *config.Config, logger *interaction.Logger) *Server {
	return &Server{cfg: cfg, logger: logger}
}

func (s *Server) Start() error {
	backend := &Backend{cfg: s.cfg, logger: s.logger}

	for _, port := range []int{s.cfg.SMTPPort, s.cfg.SMTPSPort} {
		p := port
		srv := smtp.NewServer(backend)
		srv.Addr = net.JoinHostPort("", itoa(p))
		srv.Domain = s.cfg.Domain
		srv.ReadTimeout = 30 * time.Second
		srv.WriteTimeout = 30 * time.Second
		srv.MaxMessageBytes = 10 << 20
		srv.MaxRecipients = 50
		srv.AllowInsecureAuth = true

		go func() {
			log.Printf("SMTP listening on :%d", p)
			if err := srv.ListenAndServe(); err != nil {
				log.Printf("SMTP :%d error: %v", p, err)
			}
		}()
	}
	return nil
}

type Backend struct {
	cfg    *config.Config
	logger *interaction.Logger
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{cfg: b.cfg, logger: b.logger, remote: c.Conn().RemoteAddr().String()}, nil
}

type Session struct {
	cfg      *config.Config
	logger   *interaction.Logger
	remote   string
	mailFrom string
	rcptTo   []string
	host     string
}

func (s *Session) AuthPlain(username, password string) error {
	return nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.mailFrom = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.rcptTo = append(s.rcptTo, to)
	if s.host == "" {
		parts := strings.Split(strings.ToLower(to), "@")
		if len(parts) == 2 {
			s.host = parts[1]
		}
	}
	return nil
}

func (s *Session) Data(r io.Reader) error {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(r, 10<<20)); err != nil {
		return err
	}

	host := s.host
	if host == "" {
		host = s.cfg.Domain
	}

	s.logger.LogSMTP(host, interaction.ClientIP(s.remote), s.mailFrom, s.rcptTo, buf.Bytes())
	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error { return nil }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
