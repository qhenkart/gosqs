# gosqs
https://circleci.com/gh/qhenkart/gosqs.svg?style=svg

GoSQS serves as the messaging interface between AWS-SQS and AWS-SNS services. If there is an feature you would like implemented, please make a Pull Request

Please refer to the examples to see how to interact with this library. Please make contributions, and post any issues.

Take a look at the Dead Letter Queue and Naming your Queue as they are important for this design pattern 

## TODO
- create better documentation
- implement internal testing package to avoid dependency
- create a controller that scales the worker pool up automatically depending on the message count and then kills the workers to avoid costs

## Scaling the Consumers
Each SQS consumer is ready to be scaled up as well as scaled out.

* Scaling Out: You can scale out by creating additional instances with no extra configuration required. The library and SQS is designed for multiple workers reaching into the same pool

* Scaling Up: Each consumer has a configuration variable `config.WorkerPool`. The default is set to `30`, that means there are 30 goroutines checking for messages at any given time. You can increase the amount of active threads simply by adjusting that number. Make sure to monitor CPU usage to find the right count for your application. For a local or dev environment. Reduce this number to 1 to save battery

## Configuring SNS
configuring SNS is easy, simply login to the AWS-console, navigate to SNS. Click on Topics on the sidebar and "Create New Topic". Fill in the name and display name.
* make sure to set the topic delivery policy to exponential back off

## Configuring SQS
1. Navigate to aws-sqs
2. Choose a queue Name and click on Standard Queue
3. Click configure, apply optional configurations
4. Hit Create Queue
5. in the main page, click on the newly created queue
6. Click on Queue Actions and at the bottom hit "Subscribe Queue to SNS Topic"
7. Select the SNS topic from the dropdown provided

## Naming your Queue
The naming convention for queues supported by this library follow the following syntax

<env>-<name>

## SQS Configurations  

### Default Visibility Timeout  
The default visibility timeout is responsible for managing how often a single message gets received by a consumer. A message remains in the queue until it is deleted, and any receipt of the message that does not end in deletion before the Visibility Timeout is hit is considered a "failure". As a result, the Default Visibility Timeout should exceed the maximum possible amount of time it would take for a handler to process and delete a message.  

The currently set default is 30 seconds

*note* The visibility timeout is extended for an individual message, for a maximum of 3 x the visibility timeout

### Message Retention Period
The # of days that the Queue will hold on to an unconsumed message before deleting it. Since we will always be consuming, this value is not important, the default is 4 days

### Receive Message Wait Time
The amount of time that the request will hang before returning 0 messages. This field is important as it allows us to use long-polling instead of short-polling. The default is 0 and it *should be set to the max 20 seconds to save on processing and cost*. AWS recommends using long polling over short polling

### Custom Attributes
You can add custom attributes to your SQS implementation. These are fields that exist outside of the payload body. A common practice is to include a correlationId or some sort of trackingId to track a message


### DEAD LETTER QUEUE CONFIGURATION
The following settings activate an automatic reroute to the DLQ upon repetetive failure of message processing.
* Redrive Policy must be checked
* The Dead Letter Queue name must be provided (A DLQ is just a normal SQS)
* Maximum Receives reflects the amount of times a message is received, but not deleted before it is requeued into the DLQ  
* *Including a DLQ is an absolute must, do not run a system without it our you will be vulnerable to Poison-Pill attacks*

## Consumer Configuration

### Custom Middleware
You can add custom middleware to your consumer. These will run using the adapter method before each handler is called. You can include a logger or modify the context etc

## Testing
You can set up a local SNS/SQS emulator using https://github.com/p4tin/goaws. Contributions have been added to this emulator specifically to support this library
Tests also require this to be running, I will eventually set up a ci environment that runs the emulator in a container and runs the tests
