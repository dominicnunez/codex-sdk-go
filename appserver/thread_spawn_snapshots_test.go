package appserver_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestThreadSpawnRuntimeSnapshots(t *testing.T) {
	for _, mode := range []string{"run", "conversation"} {
		t.Run(mode, func(t *testing.T) {
			process, mock := mockProcess(t)
			thread := validProcessThreadPayload("thread-1")
			thread["source"] = json.RawMessage(`{"subAgent":{"thread_spawn":{"agent_path":"worker/alpha","agent_nickname":"worker","agent_role":"researcher","depth":2,"parent_thread_id":"parent"}}}`)
			if err := mock.SetResponseData("thread/start", validProcessThreadStartResponse(thread)); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var stream *codex.Stream
			var conversation *codex.Conversation
			if mode == "conversation" {
				var err error
				conversation, err = process.StartConversation(ctx, codex.ConversationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				defer conversation.Close()
				stream = conversation.TurnStreamed(ctx, codex.TurnOptions{Prompt: "metadata"})
			} else {
				stream = process.RunStreamed(ctx, codex.RunOptions{Prompt: "metadata"})
			}
			waitForMethodCallCount(t, mock, "turn/start", 1)
			mock.InjectServerNotification(ctx, codex.Notification{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed","items":[]}}`)})
			for _, err := range stream.Events() {
				if err != nil {
					t.Fatal(err)
				}
			}
			result := stream.Result()
			if result == nil {
				t.Fatal("missing result")
			}
			assertSpawnSnapshot := func(thread codex.Thread) {
				t.Helper()
				spawn := thread.Source.Value.(codex.SessionSourceSubAgent).SubAgent.(codex.SubAgentSourceThreadSpawn)
				if spawn.ThreadSpawn.AgentPath == nil || *spawn.ThreadSpawn.AgentPath != "worker/alpha" || spawn.ThreadSpawn.AgentNickname == nil || *spawn.ThreadSpawn.AgentNickname != "worker" || spawn.ThreadSpawn.AgentRole == nil || *spawn.ThreadSpawn.AgentRole != "researcher" {
					t.Fatalf("metadata lost or aliased: %+v", spawn.ThreadSpawn)
				}
			}
			assertSpawnSnapshot(result.Thread)
			spawn := result.Thread.Source.Value.(codex.SessionSourceSubAgent).SubAgent.(codex.SubAgentSourceThreadSpawn)
			*spawn.ThreadSpawn.AgentPath = "mutated"
			*spawn.ThreadSpawn.AgentNickname = "mutated"
			*spawn.ThreadSpawn.AgentRole = "mutated"
			assertSpawnSnapshot(stream.Result().Thread)
			cached, ok := process.Client.ThreadStateSnapshot("thread-1")
			if !ok {
				t.Fatal("missing cached thread")
			}
			assertSpawnSnapshot(cached)
			if conversation != nil {
				assertSpawnSnapshot(conversation.Thread())
			}
		})
	}
}
