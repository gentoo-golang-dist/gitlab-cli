package api

type Checkpoint struct {
	ChannelValues CheckpointChannelValues `json:"channel_values"`
}

type CheckpointChannelValues struct {
	UIChatLog []CheckpointChatMessage `json:"ui_chat_log"`
}

type CheckpointChatMessage struct {
	MessageType     string        `json:"message_type"`
	MessageSubType  string        `json:"message_sub_type"`
	Content         string        `json:"content"`
	Timestamp       string        `json:"timestamp"`
	Status          string        `json:"status"`
	ContextElements []interface{} `json:"context_elements"`
}
