package argus_client

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/wbrijesh/argus_client/rpc/logs"
)

type Client struct {
	apiKey string
	client logs.LogsService
}

type ClientConfig struct {
	ApiKey  string
	BaseUrl string
}

func NewClient(config ClientConfig) *Client {
	return &Client{
		apiKey: config.ApiKey,
		client: logs.NewLogsServiceProtobufClient(config.BaseUrl, &http.Client{}),
	}
}

type LogLevel logs.LogLevel

const (
	LevelDebug LogLevel = LogLevel(logs.LogLevel_DEBUG)
	LevelInfo  LogLevel = LogLevel(logs.LogLevel_INFO)
	LevelWarn  LogLevel = LogLevel(logs.LogLevel_WARN)
	LevelError LogLevel = LogLevel(logs.LogLevel_ERROR)
	LevelFatal LogLevel = LogLevel(logs.LogLevel_FATAL)
)

type LogEntry struct {
	Level   LogLevel
	Message string
}

func (c *Client) SendLogs(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	} else if len(entries) > 30 {
		return errors.New("cannot send more than 30 logs at a time")
	}

	ctx := context.Background()

	pbLogs := make([]*logs.LogEntry, len(entries))

	for i, entry := range entries {
		pbLogs[i] = &logs.LogEntry{
			LogId:     uuid.New().String(),
			Timestamp: time.Now().Format(time.RFC3339),
			Level:     logs.LogLevel(entry.Level),
			Message:   entry.Message,
		}
	}

	_, err := c.client.SendLogs(ctx, &logs.SendLogsRequest{
		ApiKey: c.apiKey,
		Logs:   pbLogs,
	})

	return err
}

// for slog handler
type ArgusHandler struct {
	client *Client
}

func NewArgusHandler(client *Client) *ArgusHandler {
	return &ArgusHandler{client: client}
}

func (h *ArgusHandler) Handle(ctx context.Context, record slog.Record) error {
	level := map[slog.Level]LogLevel{
		slog.LevelDebug: LevelDebug,
		slog.LevelInfo:  LevelInfo,
		slog.LevelWarn:  LevelWarn,
		slog.LevelError: LevelError,
	}[record.Level]

	entry := LogEntry{
		Level:   level,
		Message: record.Message,
	}

	err := h.client.SendLogs([]LogEntry{entry})
	if err != nil {
		log.Println("Failed to send log: ", err)
	}

	return err
}

func (h *ArgusHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *ArgusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *ArgusHandler) WithGroup(name string) slog.Handler {
	return h
}
