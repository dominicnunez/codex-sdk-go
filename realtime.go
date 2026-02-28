package codex

import (
	"context"
	"encoding/json"
)

// ThreadRealtimeStartedNotification is sent when a realtime connection starts for a thread.
// Method: thread/realtime/started
type ThreadRealtimeStartedNotification struct {
	ThreadID  string  `json:"threadId"`
	SessionID *string `json:"sessionId,omitempty"`
}

// ThreadRealtimeClosedNotification is sent when a realtime connection closes for a thread.
// Method: thread/realtime/closed
type ThreadRealtimeClosedNotification struct {
	ThreadID string  `json:"threadId"`
	Reason   *string `json:"reason,omitempty"`
}

// ThreadRealtimeErrorNotification is sent when an error occurs during realtime communication.
// Method: thread/realtime/error
type ThreadRealtimeErrorNotification struct {
	ThreadID string `json:"threadId"`
	Message  string `json:"message"`
}

// ThreadRealtimeItemAddedNotification is sent when a non-audio item is added during realtime.
// Method: thread/realtime/itemAdded
type ThreadRealtimeItemAddedNotification struct {
	ThreadID string          `json:"threadId"`
	Item     json.RawMessage `json:"item"` // Open schema - any JSON value
}

// ThreadRealtimeAudioChunk contains audio data and metadata.
type ThreadRealtimeAudioChunk struct {
	Data              string  `json:"data"`                        // Base64-encoded audio bytes
	NumChannels       uint16  `json:"numChannels"`                 // Number of audio channels
	SampleRate        uint32  `json:"sampleRate"`                  // Sample rate in Hz
	SamplesPerChannel *uint32 `json:"samplesPerChannel,omitempty"` // Number of samples per channel
}

// ThreadRealtimeOutputAudioDeltaNotification is sent when audio output is streamed.
// Method: thread/realtime/outputAudio/delta
type ThreadRealtimeOutputAudioDeltaNotification struct {
	ThreadID string                   `json:"threadId"`
	Audio    ThreadRealtimeAudioChunk `json:"audio"`
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
			return
		}
		handler(params)
	})
}
