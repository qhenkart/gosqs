package gosqs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
)

var maxMessages = int64(10)

// Consumer provides an interface for receiving messages through AWS SQS and SNS
type Consumer interface {
	// Consume polls for new messages and if it finds one, decodes it, sends it to the handler and deletes it
	//
	// A message is not considered dequeued until it has been sucessfully processed and deleted. There is a 30 Second
	// delay between receiving a single message and receiving the same message. This delay can be adjusted in the AWS
	// console and can also be extended during operation. If a message is successfully received 4 times but not deleted,
	// it will be considered unprocessable and sent to the DLQ automatically
	//
	// Consume uses long-polling to check and retrieve messages, if it is unable to make a connection, the aws-SDK will use its
	// advanced retrying mechanism (including exponential backoff), if all of the retries fail, then we will wait 10s before
	// trying again.
	//
	// When a new message is received, it runs in a separate go-routine that will handle the full consuming of the message, error reporting
	// and deleting
	Consume()
	// RegisterHandler registers an event listener and an associated handler. If the event matches, the handler will
	// be run
	RegisterHandler(name string, h Handler, adapters ...Adapter)
	// Message serves as the direct messaging capability within the consumer. A worker can send direct messages to other workers
	Message(ctx context.Context, queue, event string, body interface{})
	// MessageSelf serves as the self messaging capability within the consumer, a worker can send messages to itself for continued
	// processing and resiliency
	MessageSelf(ctx context.Context, event string, body interface{})
}

// consumer is a wrapper around sqs.SQS
type consumer struct {
	sqs               *sqs.SQS
	handlers          map[string]Handler
	env               string
	QueueURL          string
	Hostname          string
	VisibilityTimeout int
	workerPool        int
	workerCount       int
	extensionLimit    int
	attributes        []customAttribute

	logger Logger
}

// NewConsumer creates a new SQS instance and provides a configured consumer interface for
// receiving and sending messages
func NewConsumer(c Config, queueName string) (Consumer, error) {
	sess, err := newSession(c)
	if err != nil {
		return nil, err
	}

	cons := &consumer{
		sqs:               sqs.New(sess),
		env:               c.Env,
		VisibilityTimeout: 30,
		workerPool:        30,
		extensionLimit:    2,
	}

	if c.Logger == nil {
		cons.logger = &defaultLogger{}
	}

	if c.VisibilityTimeout != 0 {
		cons.VisibilityTimeout = c.VisibilityTimeout
	}

	if c.WorkerPool != 0 {
		cons.workerPool = c.WorkerPool
	}

	if c.ExtensionLimit != nil {
		cons.extensionLimit = *c.ExtensionLimit
	}

	cons.QueueURL = c.QueueURL
	// custom QueueURLs can be provided for testing and mocking purposes
	if cons.QueueURL == "" {
		name := fmt.Sprintf("%s-%s", c.Env, queueName)
		o, err := cons.sqs.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: &name})
		if err != nil {
			return nil, err
		}
		cons.QueueURL = *o.QueueUrl
	}

	return cons, nil
}

// Logger accesses the logging field or applies a default logger
func (c *consumer) Logger() Logger {
	if c.logger == nil {
		return &defaultLogger{}
	}
	return c.logger
}

// RegisterHandler registers an event listener and an associated handler. If the event matches, the handler will
// be run along with any included middleware
func (c *consumer) RegisterHandler(name string, h Handler, adapters ...Adapter) {
	if c.handlers == nil {
		c.handlers = make(map[string]Handler)
	}

	for i := len(adapters) - 1; i >= 0; i-- {
		h = adapters[i](h)
	}

	c.handlers[name] = func(ctx context.Context, m Message) error {
		return h(ctx, m)
	}
}

var (
	all = "All"
)

// Consume polls for new messages and if it finds one, decodes it, sends it to the handler and deletes it
//
// A message is not considered dequeued until it has been sucessfully processed and deleted. There is a 30 Second
// delay between receiving a single message and receiving the same message. This delay can be adjusted in the AWS
// console and can also be extended during operation. If a message is successfully received 4 times but not deleted,
// it will be considered unprocessable and sent to the DLQ automatically
//
// Consume uses long-polling to check and retrieve messages, if it is unable to make a connection, the aws-SDK will use its
// advanced retrying mechanism (including exponential backoff), if all of the retries fail, then we will wait 10s before
// trying again.
//
// When a new message is received, it runs in a separate go-routine that will handle the full consuming of the message, error reporting
// and deleting
func (c *consumer) Consume() {
	jobs := make(chan *message)
	for w := 1; w <= c.workerPool; w++ {
		go c.worker(w, jobs)
	}

	for {
		output, err := c.sqs.ReceiveMessage(&sqs.ReceiveMessageInput{QueueUrl: &c.QueueURL, MaxNumberOfMessages: &maxMessages, MessageAttributeNames: []*string{&all}})
		if err != nil {
			c.Logger().Println("%s , retrying in 10s", ErrGetMessage.Context(err).Error())
			time.Sleep(10 * time.Second)
			continue
		}

		for _, m := range output.Messages {
			// If the Message doesn't have a "route" attribute, the default route is "".
			// (This allows us to run a queue without routing.)
			if m.MessageAttributes == nil {
				m.MessageAttributes = defaultSQSAttributes("")
			}
			if _, ok := m.MessageAttributes["route"]; !ok {
				t := "String"
				v := ""
				m.MessageAttributes["route"] = &sqs.MessageAttributeValue{DataType: &t, StringValue: &v}
			}

			jobs <- newMessage(m)
		}
	}
}

// worker is an always-on concurrent worker that will take tasks when they are added into the messages buffer
func (c *consumer) worker(id int, messages <-chan *message) {
	for m := range messages {
		if err := c.run(m); err != nil {
			c.Logger().Println(err.Error())
		}
	}
}

// run should be run within a worker

// if there is no handler for that route, then the message will be deleted and fully consumed
//
// if the handler exists, it will wait for the err channel to be processed. Once it receives feedback from the handler in the form
// of a channel, it will either log the error, or consume the message
func (c *consumer) run(m *message) error {
	if h, ok := c.handlers[m.Route()]; ok {
		ctx := context.Background()

		go c.extend(ctx, m)
		if err := h(ctx, m); err != nil {
			return m.ErrorResponse(ctx, err)
		}

		// finish the extension channel if the message was processed successfully
		m.Success(ctx)
	}

	//deletes message if the handler was successful or if there was no handler with that route
	return c.delete(m) //MESSAGE CONSUMED
}

// MessageSelf serves as the self messaging capability within the consumer, a worker can send messages to itself for continued
// processing and resiliency
func (c *consumer) MessageSelf(ctx context.Context, event string, body interface{}) {
	o, err := json.Marshal(body)
	if err != nil {
		log.Println(ErrMarshal.Context(err).Error(), event)
		return
	}

	out := string(o)

	sqsInput := &sqs.SendMessageInput{
		MessageBody:       &out,
		MessageAttributes: defaultSQSAttributes(event, c.attributes...),
		QueueUrl:          &c.QueueURL,
	}

	go c.sendDirectMessage(ctx, sqsInput, event)
}

// Message serves as the direct messaging capability within the consumer. A worker can send direct messages to other workers
func (c *consumer) Message(ctx context.Context, queue, event string, body interface{}) {
	name := fmt.Sprintf("%s-%s", c.env, queue)

	queueResp, err := c.sqs.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: &name})
	if err != nil {
		log.Printf("%s, queue: %s", ErrQueueURL.Context(err).Error(), name)
		return
	}

	o, err := json.Marshal(body)
	if err != nil {
		log.Println(ErrMarshal.Context(err).Error(), event)
		return
	}

	out := string(o)

	sqsInput := &sqs.SendMessageInput{
		MessageBody:       &out,
		MessageAttributes: defaultSQSAttributes(event, c.attributes...),
		QueueUrl:          queueResp.QueueUrl,
	}

	go c.sendDirectMessage(ctx, sqsInput, event)
}

// sendDirectMessage is a helper that should be run concurrently since it will block the main thread if there is a connection issue
func (c *consumer) sendDirectMessage(ctx context.Context, input *sqs.SendMessageInput, event string) {
	if _, err := c.sqs.SendMessage(input); err != nil {
		log.Printf("%s, event: %s \nretrying in 10s", ErrPublish.Context(err).Error(), event)
		time.Sleep(10 * time.Second)
		c.sendDirectMessage(ctx, input, event)
	}
}

// delete will remove a message from the queue, this is necessary to fully and successfully consume a message
func (c *consumer) delete(m *message) error {
	_, err := c.sqs.DeleteMessage(&sqs.DeleteMessageInput{QueueUrl: &c.QueueURL, ReceiptHandle: m.ReceiptHandle})
	if err != nil {
		c.Logger().Println(ErrUnableToDelete.Context(err).Error())
		return ErrUnableToDelete.Context(err)
	}
	return nil
}

func (c *consumer) extend(ctx context.Context, m *message) {
	var count int
	extension := int64(c.VisibilityTimeout)
	for {
		//only allow 1 extensions (Default 1m30s)
		if count >= c.extensionLimit {
			c.Logger().Println(ErrMessageProcessing.Error(), m.Route())
			return
		}

		count++
		// allow 10 seconds to process the extension request
		time.Sleep(time.Duration(c.VisibilityTimeout-10) * time.Second)
		select {
		case <-m.err:
			// goroutine finished
			return
		default:
			// double the allowed processing time
			extension = extension + int64(c.VisibilityTimeout)
			_, err := c.sqs.ChangeMessageVisibility(&sqs.ChangeMessageVisibilityInput{QueueUrl: &c.QueueURL, ReceiptHandle: m.ReceiptHandle, VisibilityTimeout: &extension})
			if err != nil {
				c.Logger().Println(ErrUnableToExtend.Error(), err.Error())
				return
			}
		}
	}
}
