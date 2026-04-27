package mqtt

import "github.com/go-kratos/kratos/v2/errors"

var (
	ErrNotConnected = errors.New(503, "MQTT_NOT_CONNECTED", "MQTT client is not connected")
	ErrPublishTimeout = errors.New(504, "MQTT_PUBLISH_TIMEOUT", "MQTT publish timed out")
	ErrSubscribeFailed = errors.New(503, "MQTT_SUBSCRIBE_FAILED", "MQTT subscribe failed")
	ErrInvalidTopic = errors.New(400, "MQTT_INVALID_TOPIC", "invalid MQTT topic format")
)
