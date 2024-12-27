package argus_client

import (
	"context"
	"errors"
	"net/http"

	"github.com/wbrijesh/argus_client/rpc/logs"

	"time"

	"github.com/google/uuid"
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
