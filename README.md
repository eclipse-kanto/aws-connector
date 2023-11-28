![Kanto logo](https://github.com/eclipse-kanto/kanto/raw/main/logo/kanto.svg)

# Eclipse Kanto - AWS Connector
![coverage](https://raw.githubusercontent.com/bosch-io/aws-connector/badges/.badges/218-coverage-report/coverage.svg)

[![Go Coverage](https://github.com/bosch-io/aws-connector/wiki/coverage.svg)](https://github.com/bosch-io/aws-connector/wiki/coverage.html)

The **AWS Connector** is the main coordination center which forwards the local
and remote messages. Messages processed by the **AWS Connector** will
typically be related to telemetry data from the devices or command &
control from the cloud. Messages sent to the [Twin Channel](https://eclipse.dev/ditto/protocol-twinlive.html#twin)
are processed and redirected to the [Device Shadow](https://docs.aws.amazon.com/iot/latest/developerguide/device-shadow-document.html) service.
Additionally, the connector is responsible for announcing the provisioning
thing information to the local MQTT broker subscribers.

For most of the connectivity options (e.g. certificates, alpn, tpm),
**AWS Connector** is using the [Suite Connector](https://github.com/eclipse-kanto/suite-connector)
as a library.

# Table of Contents

1. [Transform _Ditto_ message to _Shadow_ message](#transform-ditto-message-to-shadow-messages)
2. [Exclude message by _Ditto_ topic](#exclude-message-by-ditto-topic)
3. [Exclude parts of the message payload](#exclude-parts-of-the-message-payload)
4. [Example **Ditto** message sent to root thing](#example-ditto-message-sent-to-root-thing)
5. [Example **Ditto** message sent to child thing](#example-ditto-message-sent-to-child-thing)

## Transform Ditto message to Shadow messages

Messages sent to the [Twin Channel](https://eclipse.dev/ditto/protocol-twinlive.html#twin) will be processed as follows:

1. Check the message against the topic exclusion filter. See [exclude message by _Ditto_ topic](#exclude-message-by-ditto-topic).
2. Check the message payload paths against the payload exclusion filters. See [exclude parts of the message payload](#exclude-parts-of-the-message-payload)
3. Check if the message contains some attributes. Attributes of the root thing will be sent with topic:

    > $aws/things/**root-thing-name**/shadow/update

    While attributes of child things will be sent with topic:

    > $aws/things/**root-thing-name**/shadow/name/**child-thing-name**/update

4. Check if the message contains some features. Features of the root thing will be sent with topic:

    > $aws/things/**root-thing-name**/shadow/name/**feature-name**/update

    While attributes of child things will be sent with topic:

    > $aws/things/**root-thing-name**/shadow/name/**child-thing-name**:**feature-name**/update

The transformation may result in multiple smaller messages sent to the
[Device Shadow](https://docs.aws.amazon.com/iot/latest/developerguide/device-shadow-document.html)
service. See the examples [Root Thing](#example-ditto-message-sent-to-root-thing)
and [Child Thing](#example-ditto-message-sent-to-child-thing) below for more details.

## Exclude message by _Ditto_ topic

Filtering unnecessary messages can save a lot of cloud traffic/cost. **AWS Connector**
can exclude messages by their [Ditto Topic](https://eclipse.dev/ditto/protocol-specification-topic.html).
To do so, provide a regex filter via **topicFilter** command line parameter or its
corresponding **JSON** configuration.

The following example will exclude all of the  messages, for which their topic
starts with **test/device:edge:containers/**

> -topicFilter "^test/device:edge:containers/.*"

## Exclude parts of the message payload

Filtering unnecessary parts of a message can improve the cost savings and
reduce the traffic load. The **payloadFilters** command line parameter or
its corresponding **JSON** configuration can be used to remove the
unnecessary parts of it. Multiple payload filters can be provided.

The message payload is in **JSON** format and each complete **JSON** path
will be tested against all payload filters. If there is a match, the particular
part of the message will be removed.

An example **Ditto** message

```json
{
    ...
    "value":{
        "code":[
            {"value":"test"},
            {"keep":201},
            {"unwanted":500}
        ],
        "status":200,
        "unwanted":1234
    }
}
```

Applying following payload filters

> -payloadFilters ".*/unwanted$" -payloadFilters ".*/0/value$"

Will trim the message payload to

```json
{
    ...
    "value":{
        "code":[
            {"keep":201}
        ],
        "status":200
    }
}
```

## Example **Ditto** message sent to root thing

```json
{
    "topic":"ex/root/things/twin/commands/modify",
    "headers":{"response-required":false},
    "path":"/attributes/test",
    "value":{
        "attributes": {
            "location": {
                "latitude": 44.673856,
                "longitude": 8.261719
            }
        },
        "features": {
            "accelerometer": {
                "properties": {
                    "x": 3.141,
                    "y": 2.718,
                    "z": 1,
                    "unit": "g"
                }
            }
        }
    }
}
```

The root thing name is **ex:root** and following messages will be send
to [Device Shadow](https://docs.aws.amazon.com/iot/latest/developerguide/device-shadow-document.html) service:

1. Topic **$aws/things/*ex:root*/shadow/update** with value

```json
{
    "state": {
        "reported": {
            "location": {
                "latitude": 44.673856,
                "longitude": 8.261719
            }
        }
    }
}
```

2. Topic **$aws/things/*ex:root*/shadow/name/*accelerometer*/update** with value

```json
{
    "state": {
        "reported": {
            "x": 3.141,
            "y": 2.718,
            "z": 1,
            "unit": "g"
        }
    }
}
```

## Example **Ditto** message sent to child thing

```json
{
    "topic":"ex/root:child/things/twin/commands/modify",
    "headers":{"response-required":false},
    "path":"/attributes/test",
    "value":{
        "attributes": {
            "location": {
                "latitude": 44.673856,
                "longitude": 8.261719
            }
        },
        "features": {
            "accelerometer": {
                "properties": {
                    "x": 3.141,
                    "y": 2.718,
                    "z": 1,
                    "unit": "g"
                }
            }
        }
    }
}
```

The root thing name is **ex:root**, its child thing name is **child** and following
messages will be send to [Device Shadow](https://docs.aws.amazon.com/iot/latest/developerguide/device-shadow-document.html) service:

1. Topic **$aws/things/*ex:root*/shadow/name/*child*/update** with value

```json
{
    "state": {
        "reported": {
            "location": {
                "latitude": 44.673856,
                "longitude": 8.261719
            }
        }
    }
}
```

2. Topic **$aws/things/*ex:root*/shadow/name/*child:accelerometer*/update** with value

```json
{
    "state": {
        "reported": {
            "x": 3.141,
            "y": 2.718,
            "z": 1,
            "unit": "g"
        }
    }
}
```

## Community

* [GitHub Issues](https://github.com/eclipse-kanto/aws-connector/issues)
* [Mailing List](https://accounts.eclipse.org/mailing-list/kanto-dev)
