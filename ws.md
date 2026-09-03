type ModuleMessage struct {
	Type      string      `json:"type"`
	Topic     string      `json:"topic,omitempty"`
	ThreadID  string      `json:"thread_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

{
    type: "start",
    Topic: "call",
    ThreadID: "CallID",
    Data:{
        .......
    }
    }