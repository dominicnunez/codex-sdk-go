package codex

import (
	"context"
	"encoding/json"
	"fmt"
)

// RealtimeConversationVersion identifies the realtime protocol version.
type RealtimeConversationVersion string

const (
	RealtimeConversationVersionV1 RealtimeConversationVersion = "v1"
	RealtimeConversationVersionV2 RealtimeConversationVersion = "v2"
)

// ThreadRealtimeStartedNotification is sent when a realtime connection starts for a thread.
// Method: thread/realtime/started
type ThreadRealtimeStartedNotification struct {
	ThreadID  string                      `json:"threadId"`
	SessionID *string                     `json:"sessionId,omitempty"`
	Version   RealtimeConversationVersion `json:"version"`
}

func (n *ThreadRealtimeStartedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeStartedNotification
	var decoded wire
	required := []string{"threadId", "version"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadRealtimeStartedNotification(decoded)
	return nil
}

// ThreadRealtimeClosedNotification is sent when a realtime connection closes for a thread.
// Method: thread/realtime/closed
type ThreadRealtimeClosedNotification struct {
	ThreadID string  `json:"threadId"`
	Reason   *string `json:"reason,omitempty"`
}

func (n *ThreadRealtimeClosedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeClosedNotification
	var decoded wire
	required := []string{"threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadRealtimeClosedNotification(decoded)
	return nil
}

// ThreadRealtimeErrorNotification is sent when an error occurs during realtime communication.
// Method: thread/realtime/error
type ThreadRealtimeErrorNotification struct {
	ThreadID string          `json:"threadId"`
	Message  string          `json:"message"`
	Raw      json.RawMessage `json:"-"`
}

func (n *ThreadRealtimeErrorNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeErrorNotification
	var decoded wire
	required := []string{"message", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	decoded.Raw = append(json.RawMessage(nil), data...)
	*n = ThreadRealtimeErrorNotification(decoded)
	return nil
}

// ThreadRealtimeItemAddedNotification is sent when a non-audio item is added during realtime.
// Method: thread/realtime/itemAdded
type ThreadRealtimeItemAddedNotification struct {
	ThreadID string          `json:"threadId"`
	Item     json.RawMessage `json:"item"` // Open schema - any JSON value
}

func (n *ThreadRealtimeItemAddedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeItemAddedNotification
	var decoded wire
	required := []string{"item", "threadId"}
	nonNull := []string{"threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, nonNull); err != nil {
		return err
	}
	*n = ThreadRealtimeItemAddedNotification(decoded)
	return nil
}

// ThreadRealtimeAudioChunk contains audio data and metadata.
type ThreadRealtimeAudioChunk struct {
	Data              string  `json:"data"`                        // Base64-encoded audio bytes
	ItemID            *string `json:"itemId,omitempty"`            // Associated item when the chunk belongs to a specific output item
	NumChannels       uint16  `json:"numChannels"`                 // Number of audio channels
	SampleRate        uint32  `json:"sampleRate"`                  // Sample rate in Hz
	SamplesPerChannel *uint32 `json:"samplesPerChannel,omitempty"` // Number of samples per channel
}

func (c *ThreadRealtimeAudioChunk) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeAudioChunk
	var decoded wire
	required := []string{"data", "numChannels", "sampleRate"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*c = ThreadRealtimeAudioChunk(decoded)
	return nil
}

// ThreadRealtimeOutputAudioDeltaNotification is sent when audio output is streamed.
// Method: thread/realtime/outputAudio/delta
type ThreadRealtimeOutputAudioDeltaNotification struct {
	ThreadID string                   `json:"threadId"`
	Audio    ThreadRealtimeAudioChunk `json:"audio"`
}

func (n *ThreadRealtimeOutputAudioDeltaNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadRealtimeOutputAudioDeltaNotification
	var decoded wire
	required := []string{"audio", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadRealtimeOutputAudioDeltaNotification(decoded)
	return nil
}

// OnThreadRealtimeStarted registers a listener for thread/realtime/started notifications.
func (c *Client) OnThreadRealtimeStarted(handler func(ThreadRealtimeStartedNotification)) {
	if handler == nil {
		c.OnNotification(notifyRealtimeStarted, nil)
		return
	}
	c.OnNotification(notifyRealtimeStarted, func(ctx context.Context, notif Notification) {
		var params ThreadRealtimeStartedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyRealtimeStarted, fmt.Errorf("unmarshal %s: %w", notifyRealtimeStarted, err))
			return
		}
		handler(params)
	})
}

// OnThreadRealtimeClosed registers a listener for thread/realtime/closed notifications.
func (c *Client) OnThreadRealtimeClosed(handler func(ThreadRealtimeClosedNotification)) {
	if handler == nil {
		c.OnNotification(notifyRealtimeClosed, nil)
		return
	}
	c.OnNotification(notifyRealtimeClosed, func(ctx context.Context, notif Notification) {
		var params ThreadRealtimeClosedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyRealtimeClosed, fmt.Errorf("unmarshal %s: %w", notifyRealtimeClosed, err))
			return
		}
		handler(params)
	})
}

// OnThreadRealtimeError registers a listener for thread/realtime/error notifications.
func (c *Client) OnThreadRealtimeError(handler func(ThreadRealtimeErrorNotification)) {
	if handler == nil {
		c.OnNotification(notifyRealtimeError, nil)
		return
	}
	c.OnNotification(notifyRealtimeError, func(ctx context.Context, notif Notification) {
		var params ThreadRealtimeErrorNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyRealtimeError, fmt.Errorf("unmarshal %s: %w", notifyRealtimeError, err))
			return
		}
		handler(params)
	})
}

// OnThreadRealtimeItemAdded registers a listener for thread/realtime/itemAdded notifications.
func (c *Client) OnThreadRealtimeItemAdded(handler func(ThreadRealtimeItemAddedNotification)) {
	if handler == nil {
		c.OnNotification(notifyRealtimeItemAdded, nil)
		return
	}
	c.OnNotification(notifyRealtimeItemAdded, func(ctx context.Context, notif Notification) {
		var params ThreadRealtimeItemAddedNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyRealtimeItemAdded, fmt.Errorf("unmarshal %s: %w", notifyRealtimeItemAdded, err))
			return
		}
		handler(params)
	})
}

// OnThreadRealtimeOutputAudioDelta registers a listener for thread/realtime/outputAudio/delta notifications.
func (c *Client) OnThreadRealtimeOutputAudioDelta(handler func(ThreadRealtimeOutputAudioDeltaNotification)) {
	if handler == nil {
		c.OnNotification(notifyRealtimeOutputAudioDelta, nil)
		return
	}
	c.OnNotification(notifyRealtimeOutputAudioDelta, func(ctx context.Context, notif Notification) {
		var params ThreadRealtimeOutputAudioDeltaNotification
		if err := json.Unmarshal(notif.Params, &params); err != nil {
			c.reportHandlerError(notifyRealtimeOutputAudioDelta, fmt.Errorf("unmarshal %s: %w", notifyRealtimeOutputAudioDelta, err))
			return
		}
		handler(params)
	})
}
