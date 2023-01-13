package gosqs

import (
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
)

type SessionProviderFunc func(c Config) (*session.Session, error)

// Config defines the gosqs configuration
type Config struct {
	// a way to provide custom session setup. A default based on key/secret will be used if not provided
	SessionProvider SessionProviderFunc
	// private key to access aws
	Key string
	// secret to access aws
	Secret string
	// region for aws and used for determining the topic ARN
	Region string
	// provided automatically by aws, but must be set for emulators or local testing
	Hostname string
	// account ID of the aws account, used for determining the topic ARN
	AWSAccountID string
	// environment name, used for determinig the topic ARN
	Env string
	// prefix of the topic, this is set as a prefix to the environment
	TopicPrefix string
	// optional address of the topic, if this is not provided it will be created using other variables
	TopicARN string
	// optional address of queue, if this is not provided it will be retrieved during setup
	QueueURL string
	// used to extend the allowed processing time of a message
	VisibilityTimeout int
	// used to determine how many attempts exponential backoff should use before logging an error
	RetryCount int
	// defines the total amount of goroutines that can be run by the consumer
	WorkerPool int
	// defines the total number of processing extensions that occur. Each proccessing extension will double the
	// visibilitytimeout counter, ensuring the handler has more time to process the message. Default is 2 extensions (1m30s processing time)
	// set to 0 to turn off extension processing
	ExtensionLimit *int

	// Add custom attributes to the message. This might be a correlationId or client meta information
	// custom attributes will be viewable on the sqs dashboard as meta data
	Attributes []customAttribute

	// Add a custom logger, the default will be log.Println
	Logger Logger
}

// customAttribute add custom attributes to SNS and SQS messages. This can include correlationIds, or any additional information you would like
// separate from the payload body. These attributes can be easily seen from the SQS console.
type customAttribute struct {
	Title string
	// Use gosqs.DataTypeNumber or gosqs.DataTypeString
	DataType string
	// Value represents the value
	Value string
}

// NewCustomAttribute adds a custom attribute to SNS and SQS messages. This can include correlationIds, logIds, or any additional information you would like
// separate from the payload body. These attributes can be easily seen from the SQS console.
//
// must use gosqs.DataTypeNumber of gosqs.DataTypeString for the datatype, the value must match the type provided
func (c *Config) NewCustomAttribute(dataType dataType, title string, value interface{}) error {
	if dataType == DataTypeNumber {
		val, ok := value.(int)
		if !ok {
			return ErrMarshal
		}

		c.Attributes = append(c.Attributes, customAttribute{title, dataType.String(), strconv.Itoa(val)})
		return nil
	}

	val, ok := value.(string)
	if !ok {
		return ErrMarshal
	}
	c.Attributes = append(c.Attributes, customAttribute{title, dataType.String(), val})
	return nil
}

type dataType string

func (dt dataType) String() string {
	return string(dt)
}

// DataTypeNumber represents the Number datatype, use it when creating custom attributes
const DataTypeNumber = dataType("Number")

// DataTypeString represents the String datatype, use it when creating custom attributes
const DataTypeString = dataType("String")

type retryer struct {
	client.DefaultRetryer
	retryCount int
}

// MaxRetries sets the total exponential back off attempts to 10 retries
func (r retryer) MaxRetries() int {
	if r.retryCount > 0 {
		return r.retryCount
	}

	return 10
}

// newSession creates a new aws session.
// This will be used as the default SessionProvider if one is not set
func newSession(c Config) (*session.Session, error) {
	//sets credentials
	creds := credentials.NewStaticCredentials(c.Key, c.Secret, "")
	_, err := creds.Get()
	if err != nil {
		return nil, ErrInvalidCreds.Context(err)
	}

	r := &retryer{retryCount: c.RetryCount}

	cfg := request.WithRetryer(aws.NewConfig().WithRegion(c.Region).WithCredentials(creds), r)

	//if an optional hostname config is provided, then replace the default one
	//
	// This will set the default AWS URL to a hostname of your choice. Perfect for testing, or mocking functionality
	if c.Hostname != "" {
		cfg.Endpoint = &c.Hostname
	}

	return session.NewSession(cfg)
}
