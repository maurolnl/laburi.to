package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// stubAPI captura los inputs que el cliente arma y devuelve lo que el test configure. Es lo
// que permite comprobar la traducción de tipos sin red ni credenciales.
type stubAPI struct {
	sendInput    *sqs.SendMessageInput
	receiveInput *sqs.ReceiveMessageInput
	deleteInput  *sqs.DeleteMessageInput
	extendInput  *sqs.ChangeMessageVisibilityInput

	receiveOutput *sqs.ReceiveMessageOutput
	err           error
}

func (s *stubAPI) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	s.sendInput = in
	return &sqs.SendMessageOutput{}, s.err
}

func (s *stubAPI) ReceiveMessage(_ context.Context, in *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	s.receiveInput = in
	if s.err != nil {
		return nil, s.err
	}
	if s.receiveOutput == nil {
		return &sqs.ReceiveMessageOutput{}, nil
	}
	return s.receiveOutput, nil
}

func (s *stubAPI) DeleteMessage(_ context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	s.deleteInput = in
	return &sqs.DeleteMessageOutput{}, s.err
}

func (s *stubAPI) ChangeMessageVisibility(_ context.Context, in *sqs.ChangeMessageVisibilityInput, _ ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	s.extendInput = in
	return &sqs.ChangeMessageVisibilityOutput{}, s.err
}

func testConfig(t *testing.T) Config {
	t.Helper()

	cfg, err := LoadConfig(envLookup(enabledEnv()))
	if err != nil {
		t.Fatalf("LoadConfig() = %v, want nil", err)
	}

	return cfg
}

func TestNewReturnsDisabledWhenTheQueueIsOff(t *testing.T) {
	client, err := New(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	if _, ok := client.(Disabled); !ok {
		t.Fatalf("New() = %T, want Disabled", client)
	}
}

// Construir el cliente no debe hablar con AWS: el SDK abre la conexión recién en la primera
// llamada. Por eso esta prueba pasa sin credenciales ni red.
func TestNewBuildsAnSQSClientWithoutNetwork(t *testing.T) {
	env := enabledEnv()
	env[EnvEndpointURL] = "http://localhost:4566"

	cfg, err := LoadConfig(envLookup(env))
	if err != nil {
		t.Fatalf("LoadConfig() = %v, want nil", err)
	}

	client, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	if _, ok := client.(*sqsClient); !ok {
		t.Fatalf("New() = %T, want *sqsClient", client)
	}
}

func TestSendUsesTheConfiguredQueue(t *testing.T) {
	api := &stubAPI{}
	cfg := testConfig(t)
	client := &sqsClient{api: api, cfg: cfg}

	if err := client.Send(context.Background(), `{"subject":"employee"}`); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}

	if aws.ToString(api.sendInput.QueueUrl) != cfg.QueueURL {
		t.Errorf("QueueUrl = %q, want the configured queue", aws.ToString(api.sendInput.QueueUrl))
	}
	if aws.ToString(api.sendInput.MessageBody) != `{"subject":"employee"}` {
		t.Errorf("MessageBody = %q, want the body verbatim", aws.ToString(api.sendInput.MessageBody))
	}
}

// Los parámetros de recepción salen de Config y no de cada llamada: si el llamador pudiera
// elegirlos, la validación de rango del arranque quedaría sin efecto.
func TestReceiveUsesTheConfiguredParameters(t *testing.T) {
	api := &stubAPI{}
	cfg := testConfig(t)
	client := &sqsClient{api: api, cfg: cfg}

	if _, err := client.Receive(context.Background()); err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}

	if api.receiveInput.MaxNumberOfMessages != cfg.MaxMessages {
		t.Errorf("MaxNumberOfMessages = %d, want %d", api.receiveInput.MaxNumberOfMessages, cfg.MaxMessages)
	}
	if api.receiveInput.WaitTimeSeconds != cfg.WaitTimeSeconds {
		t.Errorf("WaitTimeSeconds = %d, want %d", api.receiveInput.WaitTimeSeconds, cfg.WaitTimeSeconds)
	}
	if api.receiveInput.VisibilityTimeout != cfg.VisibilityTimeoutSeconds {
		t.Errorf("VisibilityTimeout = %d, want %d", api.receiveInput.VisibilityTimeout, cfg.VisibilityTimeoutSeconds)
	}

	var asksForReceiveCount bool
	for _, attribute := range api.receiveInput.MessageSystemAttributeNames {
		if attribute == types.MessageSystemAttributeNameApproximateReceiveCount {
			asksForReceiveCount = true
		}
	}
	if !asksForReceiveCount {
		t.Error("Receive() must request the approximate receive count; the worker needs it to detect a redelivery")
	}
}

func TestReceiveTranslatesMessages(t *testing.T) {
	const receiveCountAttribute = string(types.MessageSystemAttributeNameApproximateReceiveCount)

	tests := []struct {
		name      string
		message   types.Message
		wantCount int
	}{
		{
			name: "first delivery",
			message: types.Message{
				MessageId:     aws.String("id-1"),
				Body:          aws.String("body-1"),
				ReceiptHandle: aws.String("receipt-1"),
				Attributes:    map[string]string{receiveCountAttribute: "1"},
			},
			wantCount: 1,
		},
		{
			name: "redelivery",
			message: types.Message{
				MessageId:     aws.String("id-1"),
				Body:          aws.String("body-1"),
				ReceiptHandle: aws.String("receipt-2"),
				Attributes:    map[string]string{receiveCountAttribute: "4"},
			},
			wantCount: 4,
		},
		{
			name: "absent receive count",
			message: types.Message{
				MessageId:     aws.String("id-1"),
				Body:          aws.String("body-1"),
				ReceiptHandle: aws.String("receipt-1"),
			},
			wantCount: 0,
		},
		{
			name: "unparseable receive count",
			message: types.Message{
				MessageId:     aws.String("id-1"),
				Body:          aws.String("body-1"),
				ReceiptHandle: aws.String("receipt-1"),
				Attributes:    map[string]string{receiveCountAttribute: "many"},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &stubAPI{receiveOutput: &sqs.ReceiveMessageOutput{Messages: []types.Message{tt.message}}}
			client := &sqsClient{api: api, cfg: testConfig(t)}

			messages, err := client.Receive(context.Background())
			if err != nil {
				t.Fatalf("Receive() = %v, want nil", err)
			}
			if len(messages) != 1 {
				t.Fatalf("Receive() returned %d messages, want 1", len(messages))
			}

			got := messages[0]
			if got.ID != aws.ToString(tt.message.MessageId) {
				t.Errorf("ID = %q, want %q", got.ID, aws.ToString(tt.message.MessageId))
			}
			if got.Body != aws.ToString(tt.message.Body) {
				t.Errorf("Body = %q, want %q", got.Body, aws.ToString(tt.message.Body))
			}
			if got.ReceiptHandle != aws.ToString(tt.message.ReceiptHandle) {
				t.Errorf("ReceiptHandle = %q, want %q", got.ReceiptHandle, aws.ToString(tt.message.ReceiptHandle))
			}
			if got.ReceiveCount != tt.wantCount {
				t.Errorf("ReceiveCount = %d, want %d", got.ReceiveCount, tt.wantCount)
			}
		})
	}
}

func TestReceiveReturnsAnEmptyBatchWithoutError(t *testing.T) {
	client := &sqsClient{api: &stubAPI{}, cfg: testConfig(t)}

	messages, err := client.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(messages) != 0 {
		t.Fatalf("Receive() returned %d messages, want none", len(messages))
	}
}

func TestDeleteAndExtendVisibilityForwardTheReceiptHandle(t *testing.T) {
	api := &stubAPI{}
	cfg := testConfig(t)
	client := &sqsClient{api: api, cfg: cfg}
	ctx := context.Background()

	if err := client.Delete(ctx, "receipt-1"); err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}
	if aws.ToString(api.deleteInput.ReceiptHandle) != "receipt-1" {
		t.Errorf("ReceiptHandle = %q, want receipt-1", aws.ToString(api.deleteInput.ReceiptHandle))
	}
	if aws.ToString(api.deleteInput.QueueUrl) != cfg.QueueURL {
		t.Error("Delete() must target the configured queue")
	}

	if err := client.ExtendVisibility(ctx, "receipt-1", 120); err != nil {
		t.Fatalf("ExtendVisibility() = %v, want nil", err)
	}
	if aws.ToString(api.extendInput.ReceiptHandle) != "receipt-1" {
		t.Errorf("ReceiptHandle = %q, want receipt-1", aws.ToString(api.extendInput.ReceiptHandle))
	}
	if api.extendInput.VisibilityTimeout != 120 {
		t.Errorf("VisibilityTimeout = %d, want 120", api.extendInput.VisibilityTimeout)
	}
}

func TestOperationsWrapTheUnderlyingError(t *testing.T) {
	sentinel := errors.New("aws is unreachable")
	client := &sqsClient{api: &stubAPI{err: sentinel}, cfg: testConfig(t)}
	ctx := context.Background()

	if err := client.Send(ctx, "body"); !errors.Is(err, sentinel) {
		t.Errorf("Send() = %v, want it to wrap the underlying error", err)
	}
	if _, err := client.Receive(ctx); !errors.Is(err, sentinel) {
		t.Errorf("Receive() = %v, want it to wrap the underlying error", err)
	}
	if err := client.Delete(ctx, "receipt"); !errors.Is(err, sentinel) {
		t.Errorf("Delete() = %v, want it to wrap the underlying error", err)
	}
	if err := client.ExtendVisibility(ctx, "receipt", 30); !errors.Is(err, sentinel) {
		t.Errorf("ExtendVisibility() = %v, want it to wrap the underlying error", err)
	}
}
