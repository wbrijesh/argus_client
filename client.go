package argus_client

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

// Custom slog.Handler implementation with batching and graceful shutdown
type ArgusHandler struct {
	client      *Client
	logBuffer   []LogEntry
	bufferMutex sync.Mutex
	flushTicker *time.Ticker
	stopChan    chan struct{}
}

func NewArgusHandler(client *Client) *ArgusHandler {
	handler := &ArgusHandler{
		client:      client,
		logBuffer:   make([]LogEntry, 0, 30),
		flushTicker: time.NewTicker(5 * time.Second), // Adjust the interval as needed
		stopChan:    make(chan struct{}),
	}

	go handler.startBatching()
	handler.setupSignalHandler()

	return handler
}

func (h *ArgusHandler) startBatching() {
	for {
		select {
		case <-h.flushTicker.C:
			h.flushLogs()
		case <-h.stopChan:
			h.flushLogs()
			return
		}
	}
}

func (h *ArgusHandler) flushLogs() {
	h.bufferMutex.Lock()
	defer h.bufferMutex.Unlock()

	if len(h.logBuffer) == 0 {
		return
	}

	err := h.client.SendLogs(h.logBuffer)
	if err != nil {
		log.Println("Failed to send logs: ", err)
	}

	h.logBuffer = h.logBuffer[:0] // Reset the buffer
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

	h.bufferMutex.Lock()
	h.logBuffer = append(h.logBuffer, entry)
	if len(h.logBuffer) >= 30 {
		go h.flushLogs()
	}
	h.bufferMutex.Unlock()

	return nil
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

func (h *ArgusHandler) setupSignalHandler() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		h.flushTicker.Stop()
		close(h.stopChan)
	}()
}

func (h *ArgusHandler) Flush() {
	h.flushTicker.Stop()
	h.flushLogs()
}
