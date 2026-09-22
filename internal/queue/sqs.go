package queue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// sqsAPI es la porción de la API de SQS que este paquete usa. Existe para que la traducción
// entre los tipos del SDK y los del paquete se pueda probar sin red ni credenciales.
type sqsAPI interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, in *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

type sqsClient struct {
	api sqsAPI
	cfg Config
}

var _ Client = (*sqsClient)(nil)

// New construye el acceso al transporte según la configuración: Disabled cuando la cola no
// está habilitada, y la implementación sobre SQS cuando lo está.
//
// Devuelve error en vez de abortar el proceso: quién termina el arranque es cmd, no este
// paquete. La construcción no abre ninguna conexión; el SDK la establece en la primera
// llamada.
func New(ctx context.Context, cfg Config) (Client, error) {
	if !cfg.Enabled {
		return Disabled{}, nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithRetryMaxAttempts(cfg.MaxRetryAttempts),
	)
	if err != nil {
		// El error del SDK puede describir la cadena de credenciales; no se envuelve con
		// datos de configuración para no sumarle nada que no deba estar en un log.
		return nil, fmt.Errorf("loading aws configuration for the recommendation queue: %w", err)
	}

	api := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}
	})

	return &sqsClient{api: api, cfg: cfg}, nil
}

func (c *sqsClient) Send(ctx context.Context, body string) error {
	_, err := c.api.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.cfg.QueueURL),
		MessageBody: aws.String(body),
	})
	if err != nil {
		return fmt.Errorf("sending recommendation queue message: %w", err)
	}

	return nil
}

func (c *sqsClient) Receive(ctx context.Context) ([]Message, error) {
	out, err := c.api.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.cfg.QueueURL),
		MaxNumberOfMessages: c.cfg.MaxMessages,
		WaitTimeSeconds:     c.cfg.WaitTimeSeconds,
		VisibilityTimeout:   c.cfg.VisibilityTimeoutSeconds,
		MessageSystemAttributeNames: []types.MessageSystemAttributeName{
			types.MessageSystemAttributeNameApproximateReceiveCount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("receiving recommendation queue messages: %w", err)
	}

	messages := make([]Message, 0, len(out.Messages))
	for _, message := range out.Messages {
		messages = append(messages, translateMessage(message))
	}

	return messages, nil
}

func (c *sqsClient) Delete(ctx context.Context, receiptHandle string) error {
	_, err := c.api.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.cfg.QueueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	if err != nil {
		return fmt.Errorf("deleting recommendation queue message: %w", err)
	}

	return nil
}

func (c *sqsClient) ExtendVisibility(ctx context.Context, receiptHandle string, seconds int32) error {
	_, err := c.api.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(c.cfg.QueueURL),
		ReceiptHandle:     aws.String(receiptHandle),
		VisibilityTimeout: seconds,
	})
	if err != nil {
		return fmt.Errorf("extending recommendation queue message visibility: %w", err)
	}

	return nil
}

// translateMessage pasa del tipo del SDK al del paquete. Un conteo de recepciones ausente o
// ilegible queda en cero en vez de romper la traducción: es un atributo aproximado y
// perderlo no justifica descartar un mensaje que sí hay que procesar.
func translateMessage(message types.Message) Message {
	receiveCount := 0
	if raw, ok := message.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)]; ok {
		if parsed, err := strconv.Atoi(raw); err == nil {
			receiveCount = parsed
		}
	}

	return Message{
		ID:            aws.ToString(message.MessageId),
		Body:          aws.ToString(message.Body),
		ReceiptHandle: aws.ToString(message.ReceiptHandle),
		ReceiveCount:  receiveCount,
	}
}
