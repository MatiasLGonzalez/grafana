package notifications

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	gomail "gopkg.in/mail.v2"

	"github.com/grafana/grafana/pkg/setting"
)

var tracer = otel.Tracer("github.com/grafana/grafana/pkg/services/notifications")

type SmtpClient struct {
	cfg setting.SmtpSettings
}

func ProvideSmtpService(cfg *setting.Cfg) (Mailer, error) {
	return NewSmtpClient(cfg.Smtp)
}

func NewSmtpClient(cfg setting.SmtpSettings) (*SmtpClient, error) {
	client := &SmtpClient{
		cfg: cfg,
	}

	return client, nil
}

func (sc *SmtpClient) Send(ctx context.Context, messages ...*Message) (int, error) {
	ctx, span := tracer.Start(ctx, "notifications.SmtpClient.Send",
		trace.WithAttributes(attribute.Int("messages", len(messages))),
	)
	defer span.End()

	sentEmailsCount := 0
	dialer, err := sc.createDialer()
	if err != nil {
		return sentEmailsCount, err
	}

	for _, msg := range messages {
		span.SetAttributes(
			attribute.String("smtp.sender", msg.From),
			attribute.StringSlice("smtp.recipients", msg.To),
		)

		m := sc.buildEmail(ctx, msg)

		innerError := dialer.DialAndSend(m)
		emailsSentTotal.Inc()
		if innerError != nil {
			// As gomail does not returned typed errors we have to parse the error
			// to catch invalid error when the address is invalid.
			// https://github.com/go-gomail/gomail/blob/81ebce5c23dfd25c6c67194b37d3dd3f338c98b1/send.go#L113
			if !strings.HasPrefix(innerError.Error(), "gomail: invalid address") {
				emailsSentFailed.Inc()
			}

			err = fmt.Errorf("failed to send notification to email addresses: %s: %w", strings.Join(msg.To, ";"), innerError)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			continue
		}

		sentEmailsCount++
	}

	return sentEmailsCount, err
}

// buildEmail converts the Message DTO to a gomail message.
func (sc *SmtpClient) buildEmail(ctx context.Context, msg *Message) *gomail.Message {
	m := gomail.NewMessage()
	// add all static headers to the email message
	for h, val := range sc.cfg.StaticHeaders {
		m.SetHeader(h, val)
	}
	m.SetHeader("From", msg.From)
	m.SetHeader("To", msg.To...)
	m.SetHeader("Subject", msg.Subject)

	if sc.cfg.EnableTracing {
		otel.GetTextMapPropagator().Inject(ctx, gomailHeaderCarrier{m})
	}

	sc.setFiles(m, msg)
	for _, replyTo := range msg.ReplyTo {
		m.SetAddressHeader("Reply-To", replyTo, "")
	}
	// loop over content types from settings in reverse order as they are ordered in according to descending
	// preference while the alternatives should be ordered according to ascending preference
	for i := len(sc.cfg.ContentTypes) - 1; i >= 0; i-- {
		if i == len(sc.cfg.ContentTypes)-1 {
			m.SetBody(sc.cfg.ContentTypes[i], msg.Body[sc.cfg.ContentTypes[i]])
		} else {
			m.AddAlternative(sc.cfg.ContentTypes[i], msg.Body[sc.cfg.ContentTypes[i]])
		}
	}

	return m
}

// setFiles attaches files in various forms.
func (sc *SmtpClient) setFiles(
	m *gomail.Message,
	msg *Message,
) {
	for _, file := range msg.EmbeddedFiles {
		m.Embed(file)
	}

	for _, file := range msg.AttachedFiles {
		file := file
		m.Attach(file.Name, gomail.SetCopyFunc(func(writer io.Writer) error {
			_, err := writer.Write(file.Content)
			return err
		}))
	}
}

func (sc *SmtpClient) createDialer() (*gomail.Dialer, error) {
	host, port, err := net.SplitHostPort(sc.cfg.Host)
	if err != nil {
		return nil, err
	}
	iPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	tlsconfig := &tls.Config{
		InsecureSkipVerify: sc.cfg.SkipVerify,
		ServerName:         host,
	}

	if sc.cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(sc.cfg.CertFile, sc.cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("could not load cert or key file: %w", err)
		}
		tlsconfig.Certificates = []tls.Certificate{cert}
	}

	d := gomail.NewDialer(host, iPort, sc.cfg.User, sc.cfg.Password)
	d.TLSConfig = tlsconfig
	d.StartTLSPolicy = getStartTLSPolicy(sc.cfg.StartTLSPolicy)

	if sc.cfg.EhloIdentity != "" {
		d.LocalName = sc.cfg.EhloIdentity
	} else {
		d.LocalName = setting.InstanceName
	}
	return d, nil
}

func getStartTLSPolicy(policy string) gomail.StartTLSPolicy {
	switch policy {
	case "NoStartTLS":
		return -1
	case "MandatoryStartTLS":
		return 1
	default:
		return 0
	}
}

type gomailHeaderCarrier struct {
	*gomail.Message
}

var _ propagation.TextMapCarrier = (*gomailHeaderCarrier)(nil)

func (c gomailHeaderCarrier) Get(key string) string {
	if hdr := c.Message.GetHeader(key); len(hdr) > 0 {
		return hdr[0]
	}

	return ""
}

func (c gomailHeaderCarrier) Set(key string, value string) {
	c.Message.SetHeader(key, value)
}

func (c gomailHeaderCarrier) Keys() []string {
	// there's no way to get all the header keys directly from a gomail.Message,
	// but we can encode the whole message and re-parse. This is not ideal, but
	// this function shouldn't be used in the hot path.
	buf := bytes.Buffer{}
	_, _ = c.Message.WriteTo(&buf)
	hdr, _ := textproto.NewReader(bufio.NewReader(&buf)).ReadMIMEHeader()
	keys := make([]string, 0, len(hdr))
	for k := range hdr {
		keys = append(keys, k)
	}

	return keys
}
