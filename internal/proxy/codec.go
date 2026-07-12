package proxy

import "fmt"

type frame struct{ payload []byte }

type rawCodec struct{}

func (rawCodec) Name() string { return "proxy-raw" }

func (rawCodec) Marshal(value any) ([]byte, error) {
	message, ok := value.(*frame)
	if !ok {
		return nil, fmt.Errorf("proxy codec: unexpected message %T", value)
	}
	return message.payload, nil
}

func (rawCodec) Unmarshal(data []byte, value any) error {
	message, ok := value.(*frame)
	if !ok {
		return fmt.Errorf("proxy codec: unexpected message %T", value)
	}
	message.payload = append(message.payload[:0], data...)
	return nil
}
