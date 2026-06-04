package ctxt

import (
	"encoding/json"

	"github.com/nexxia-ai/aigentic/ai"
)

type messageJSON struct {
	Type          string            `json:"type"`
	UserMessage   *ai.UserMessage   `json:"user_message,omitempty"`
	AIMessage     *ai.AIMessage     `json:"ai_message,omitempty"`
	ToolMessage   *ai.ToolMessage   `json:"tool_message,omitempty"`
	SystemMessage *ai.SystemMessage `json:"system_message,omitempty"`
}

func messageToJSON(msg ai.Message) *messageJSON {
	if msg == nil {
		return nil
	}
	mj := &messageJSON{}
	switch m := msg.(type) {
	case ai.UserMessage:
		mj.Type = "user_message"
		mj.UserMessage = &m
	case ai.AIMessage:
		mj.Type = "ai_message"
		mj.AIMessage = &m
	case ai.ToolMessage:
		mj.Type = "tool_message"
		mj.ToolMessage = &m
	case ai.SystemMessage:
		mj.Type = "system_message"
		mj.SystemMessage = &m
	default:
		return nil
	}
	return mj
}

func jsonToMessage(mj *messageJSON) ai.Message {
	if mj == nil {
		return nil
	}
	switch mj.Type {
	case "user_message":
		if mj.UserMessage != nil {
			return *mj.UserMessage
		}
	case "ai_message":
		if mj.AIMessage != nil {
			return *mj.AIMessage
		}
	case "tool_message":
		if mj.ToolMessage != nil {
			return *mj.ToolMessage
		}
	case "system_message":
		if mj.SystemMessage != nil {
			return *mj.SystemMessage
		}
	}
	return nil
}

func marshalMessageLine(msg ai.Message) ([]byte, error) {
	return json.Marshal(messageToJSON(msg))
}

func unmarshalMessageLine(data []byte) (ai.Message, error) {
	var mj messageJSON
	if err := json.Unmarshal(data, &mj); err != nil {
		return nil, err
	}
	return jsonToMessage(&mj), nil
}
