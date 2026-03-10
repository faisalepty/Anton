package memory

type Memory struct {
	// add fields as needed
}

func NewMemory() *Memory {
	return &Memory{
		// initialize fields as needed
	}
}

func (m *Memory) Store(key string, value interface{}) {
	// implement storage logic
}

func (m *Memory) Retrieve_recent() []interface{} {
	// implement retrieval logic
	return nil
}

func (m *Memory) Summarize() string {
	// implement summarization logic
	return ""
}