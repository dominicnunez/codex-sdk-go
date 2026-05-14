package codex_test

func validProcessThreadPayload(threadID string) map[string]interface{} {
	return map[string]interface{}{
		"id":            threadID,
		"cliVersion":    "1.0.0",
		"createdAt":     1700000000,
		"cwd":           "/tmp",
		"modelProvider": "openai",
		"preview":       "",
		"source":        "exec",
		"status":        map[string]interface{}{"type": "idle"},
		"turns":         []interface{}{},
		"updatedAt":     1700000000,
		"ephemeral":     true,
	}
}

func validProcessThreadStartResponse(thread map[string]interface{}) map[string]interface{} {
	response := validThreadLifecycleResponse(thread)
	response["approvalPolicy"] = "never"
	response["cwd"] = "/tmp"
	response["model"] = "o3"
	response["modelProvider"] = "openai"
	response["sandbox"] = map[string]interface{}{"type": "readOnly"}
	return response
}
