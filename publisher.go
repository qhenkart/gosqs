package gosqs

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sqs"
)

const maxRetryCount = 5

var errDataLimit = errors.New("InvalidParameterValue: One or more parameters are invalid. Reason: Message must be shorter than 262144 bytes")

// Notifier used for broadcasting messages
type Notifier interface {
	ModelName() string
}

// Publisher provides an interface for sending messages through AWS SQS and SNS
type Publisher interface {
	// Create sends a message using a notifier, the modelname will be prepended to the static event, e.g card_created
	Create(n Notifier)
	// Delete sends a message using a notifier, the modelname will be prepended to the static event, e.g card_deleted
	Delete(n Notifier)
	// Update sends a message using a notifier, the modelname will be prepended to the static event, e.g card_updated
	Update(n Notifier)
	// Modify sends a message using a notifier, as a map of changes. The modelname will be prepended to the static event, e.g card_modified
	//
	// a special decoder will need to be used to process these events
	Modify(n Notifier, changes interface{})
	// Dispatch sends a message using a notifier, the modelname will be prepended to the provided event, e.g card_published
	Dispatch(n Notifier, event string)
	// Message sends a direct message to an individual queue, the queueName(receiver) must be provided. The event will be sent
	// as is, no prepending will take place. No other queues will receive this message.
	Message(queue, message string, body interface{})
}

type publisher struct {
	sqs *sqs.SQS
	sns *sns.SNS

	arn    string
	env    string
	sqsURL string

	camelCase  bool
	attributes []customAttribute
	logger     Logger
}

// NewPublisher creates a new SQS/SNS publisher instance
func NewPublisher(c Config) (Publisher, error) {
	sess, err := newSession(c)
	if err != nil {
		return nil, err
	}

	arn := c.TopicARN
	if arn == "" {
		arn = fmt.Sprintf("arn:aws:sns:%s:%s:%s-%s", c.Region, c.AWSAccountID, c.TopicPrefix, c.Env)
	}

	sqsURL := fmt.Sprintf("%s/", c.Hostname)
	if c.Hostname == "" {
		sqsURL = fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/", c.Region, c.AWSAccountID)
	}

	if c.Logger == nil {
		c.Logger = &defaultLogger{}
	}

	pub := &publisher{
		sqs:    sqs.New(sess),
		sns:    sns.New(sess),
		arn:    arn,
		env:    c.Env,
		sqsURL: sqsURL,
	}

	return pub, nil
}

func (p *publisher) event(n Notifier, action string) string {
	if p.camelCase {
		return fmt.Sprintf("%s%s", n.ModelName(), strings.Title(action))
	}

	return fmt.Sprintf("%s_%s", n.ModelName(), action)
}

// Create sends a message using a notifier, the modelname will be prepended to the static event, e.g card_created
func (p *publisher) Create(n Notifier) {
	e := p.event(n, "created")
	go p.send(n, e)
}

// Delete sends a message using a notifier, the modelname will be prepended to the static event, e.g card_deleted
func (p *publisher) Delete(n Notifier) {
	e := p.event(n, "deleted")
	go p.send(n, e)
}

// Update sends a message using a notifier, the modelname will be prepended to the static event, e.g card_updated
func (p *publisher) Update(n Notifier) {
	e := p.event(n, "updated")
	go p.send(n, e)
}

type modify struct {
	Notifier `json:"body"`
	Changes  interface{} `json:"changes"`
}

// newModify creates a new struct with both Notifier and changes
func newModify(n Notifier, changes interface{}) *modify {
	return &modify{
		Notifier: n,
		Changes:  changes,
	}
}

// Modify sends a message using a notifier, as a map of changes. The modelname will be prepended to the static event, e.g card_modified
//
// a special decoder will need to be used to process these events
func (p *publisher) Modify(n Notifier, changes interface{}) {
	e := p.event(n, "modified")
	go p.send(newModify(n, changes), e)
}

// Dispatch sends a message using a notifier, the modelname will be prepended to the provided event, e.g card_published
func (p *publisher) Dispatch(n Notifier, event string) {
	e := p.event(n, event)
	go p.send(n, e)
}

// Message sends a direct message to an individual queue, the queueName(receiver) must be provided. The event will be sent
// as is, no prepending will take place. No other queues will receive this message.
func (p *publisher) Message(queue, event string, body interface{}) {
	name := fmt.Sprintf("%s-%s", p.env, queue)

	o, err := json.Marshal(body)
	if err != nil {
		p.logger.Println(ErrMarshal.Context(err).Error())
		return
	}

	out := string(o)

	u := p.sqsURL + name

	sqsInput := &sqs.SendMessageInput{
		MessageBody:       &out,
		MessageAttributes: defaultSQSAttributes(event, p.attributes...),
		QueueUrl:          &u,
	}

	go p.sendDirectMessage(sqsInput, event)
}

// sendDirectMessage is used to handle sending and error failures in a separate go-routine
//
// AWS-SDK will use their own retry mechanism for a failed request utilizing exponential backoff. If they fail
// then we will wait 10 seconds before trying again
func (p *publisher) sendDirectMessage(input *sqs.SendMessageInput, event string, retryCount ...int) {
	var c int
	if len(retryCount) != 0 {
		c = retryCount[0]
	}

	if c > maxRetryCount {
		return
	}

	if _, err := p.sqs.SendMessage(input); err != nil {
		if err.Error() == errDataLimit.Error() {
			panic(ErrBodyOverflow.Context(err))
		}

		log.Print(ErrPublish)
		time.Sleep(10 * time.Second)
		p.sendDirectMessage(input, event, c+1)
	}
}

// send is used to handle sending and error failures in a separate go-routine for SNS messages
//
// AWS-SDK will use their own retry mechanism for a failed request utilizing exponential backoff. If they fail
// then we will wait 10 seconds before trying again
func (p *publisher) send(body interface{}, event string, retryCount ...int) {
	var c int
	if len(retryCount) != 0 {
		c = retryCount[0]
	}

	if c > maxRetryCount {
		return
	}

	o, err := json.Marshal(body)
	if err != nil {
		panic(ErrMarshal.Context(err))
	}

	out := string(o)
	snsInput := &sns.PublishInput{Message: &out,
		MessageAttributes: defaultSNSAttributes(event, p.attributes...),
		TopicArn:          &p.arn,
	}

	var retrier func(input *sns.PublishInput, retryCount int)

	retrier = func(input *sns.PublishInput, retryCount int) {
		if c > maxRetryCount {
			return
		}

		_, err = p.sns.Publish(snsInput)
		if err != nil {
			if err.Error() == errDataLimit.Error() {
				panic(ErrBodyOverflow.Context(err).Error())
			}

			log.Println(ErrPublish.Context(err), " retrying in 10s")
			time.Sleep(10 * time.Second)
			retrier(input, retryCount+1)
			return
		}
	}

	retrier(snsInput, 0)
}

// defaultSNSAttributes provides general SNS attributes that we need for every message
func defaultSNSAttributes(event string, ca ...customAttribute) map[string]*sns.MessageAttributeValue {
	st := "String"
	m := map[string]*sns.MessageAttributeValue{
		"route": &sns.MessageAttributeValue{DataType: &st, StringValue: &event},
	}

	for _, attr := range ca {
		m[attr.Title] = &sns.MessageAttributeValue{DataType: &attr.DataType, StringValue: &attr.Value}
	}

	return m
}

// defaultSQSAttributes provides general SQS attributes that we need for every message
func defaultSQSAttributes(event string, ca ...customAttribute) map[string]*sqs.MessageAttributeValue {
	st := "String"
	m := map[string]*sqs.MessageAttributeValue{
		"route": &sqs.MessageAttributeValue{DataType: &st, StringValue: &event},
	}

	for _, attr := range ca {
		m[attr.Title] = &sqs.MessageAttributeValue{DataType: &attr.DataType, StringValue: &attr.Value}
	}

	return m
}
