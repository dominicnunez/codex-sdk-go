package codex

// dispatch.go implements the message routing layer for the Client.
//
// The Codex protocol is bidirectional JSON-RPC 2.0:
// - Client → Server: Requests (expect response) and Notifications (fire-and-forget)
// - Server → Client: Requests (approval flows) and Notifications (streaming events)
//
// This file contains the internal message router that dispatches incoming
// server→client messages to the appropriate handlers based on the `method` field.
//
// # Notification Dispatch
//
// Server→client notifications are routed to registered listeners via OnNotification().
// Each service (Thread, Turn, Account, etc.) registers listeners for its notification
// methods. Unknown notification methods are silently ignored (no error, no panic).
//
// Example notification flow:
//  1. Server sends: {"jsonrpc":"2.0", "method":"thread/closed", "params":{"threadId":"123"}}
//  2. Transport calls Client.handleNotification()
//  3. handleNotification looks up the method in notificationListeners map
//  4. If found, unmarshals params and calls the registered listener
//  5. If not found, silently ignores (unknown methods are expected as protocol evolves)
//
// # Request Dispatch (Approval Handlers)
//
// Server→client requests require a response from the client (approval flows).
// These are routed to approval handlers set via SetApprovalHandlers().
// Unknown request methods return a JSON-RPC method-not-found error (-32601).
//
// Example request flow:
//  1. Server sends: {"jsonrpc":"2.0", "method":"applyPatchApproval", "id":1, "params":{...}}
//  2. Transport calls Client.handleRequest()
//  3. handleRequest routes based on method to the appropriate handler
//  4. If no handler is set, returns {"error":{"code":-32601, "message":"Method not found"}}
//  5. If handler is set, unmarshals params, calls handler, marshals result, returns response
//
// # Thread Safety
//
// Both handleNotification and handleRequest are called concurrently from the transport
// layer (goroutines). The Client uses mutexes to protect shared state:
// - notificationListeners map is protected by listenersMu RWMutex
// - approvalHandlers struct is protected by approvalMu RWMutex
//
// # Error Handling
//
// - Notification unmarshal errors: silently ignored (bad data doesn't crash the client)
// - Request unmarshal errors: return JSON-RPC internal error (-32603)
// - Handler errors: propagated in the response error field
// - Unknown methods: ignored for notifications, method-not-found for requests

// handleNotification is the internal handler registered with the transport.
// It routes incoming server→client notifications to the appropriate registered listener.
//
// This method is already implemented in client.go. It:
// 1. Acquires a read lock on notificationListeners
// 2. Looks up the handler by notification.Method
// 3. If found, calls the handler (which unmarshals and invokes user callback)
// 4. If not found, silently ignores (unknown notifications are benign)
//
// See client.go for the implementation.

// handleRequest is the internal handler registered with the transport.
// It routes incoming server→client requests (approval flows) to the appropriate handler.
//
// This method is already implemented in client.go. It:
// 1. Acquires a read lock on approvalHandlers
// 2. Routes based on request.Method using a switch statement
// 3. If no handler is set, returns methodNotFoundResponse
// 4. If handler is set, unmarshals params, calls handler, marshals result
// 5. Returns Response with ID matching request.ID
//
// See client.go for the implementation.

// Supported notification methods (server → client, fire-and-forget):
//
// Thread notifications:
//   - thread/started
//   - thread/closed
//   - thread/archived
//   - thread/unarchived
//   - thread/nameUpdated
//   - thread/statusChanged
//   - thread/tokenUsageUpdated
//
// Turn notifications:
//   - turn/started
//   - turn/completed
//   - turn/planUpdated
//   - turn/diffUpdated
//
// Account notifications:
//   - account/updated
//   - account/loginCompleted
//   - account/rateLimitsUpdated
//
// Config notifications:
//   - config/warning
//
// Model notifications:
//   - model/rerouted
//
// Skills notifications: (none)
//
// Apps notifications:
//   - app/listUpdated
//
// MCP notifications:
//   - mcp/server/oauthLoginCompleted
//   - mcp/tool/callProgress
//
// Command notifications:
//   - command/executionOutputDelta
//
// Streaming notifications:
//   - agent/messageDelta
//   - turn/fileChangeOutputDelta
//   - turn/planDelta
//   - turn/reasoningTextDelta
//   - turn/reasoningSummaryTextDelta
//   - turn/reasoningSummaryPartAdded
//   - turn/itemStarted
//   - turn/itemCompleted
//   - turn/rawResponseItemCompleted
//
// Realtime notifications:
//   - thread/realtime/started
//   - thread/realtime/closed
//   - thread/realtime/error
//   - thread/realtime/itemAdded
//   - thread/realtime/outputAudio/delta
//
// Fuzzy search notifications:
//   - fuzzyFileSearch/sessionCompleted
//   - fuzzyFileSearch/sessionUpdated
//
// System notifications:
//   - windowsSandbox/setupCompleted
//   - windows/worldWritableWarning
//   - thread/compacted (DEPRECATED)
//   - deprecationNotice
//   - error
//   - item/commandExecution/terminalInteraction
//
// Supported request methods (server → client, expect response):
//
// Approval flows:
//   - applyPatchApproval
//   - item/commandExecution/requestApproval
//   - execCommandApproval
//   - item/fileChange/requestApproval
//   - skill/requestApproval
//   - item/tool/call (DynamicToolCall)
//   - item/tool/requestUserInput
//   - fuzzyFileSearch
//   - account/chatgptAuthTokens/refresh
