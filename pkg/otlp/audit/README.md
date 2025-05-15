# OpenTelemetry Event Library

**Go** library for creating and sending audit log events to OpenTelemetry collector using HTTP requests.

## Dependencies

* **go 1.23** or higher
* Working OpenTelemetry collector instance with a HTTP receiver

## How to use
### Configuration
Configuration is being read from `config.yaml` file. The consumers should use `LoadConfig` function provided within `common` package to load and unmarshal configuration into a struct to be used by consumers. The section related to this library in consumer's config should look like this:
```
audit:
  endpoint: "<YOUR_ENDPOINT>"
  # potential auth config
```
The library allows two ways of authenticating against the target endpoint: **mTLS** and **Basic Auth**. If needed, they should be configured within the config.yaml. Please note that only one authenticating method can be implemented at a time. If both are provided, mTLS is prioritized. Example config snippets:
```
  basicAuth:
    username:
      source: file
      file:
        path: <JSON_FILE>
        format: json
        jsonPath: "$.<KEY_FOR_USERNAME_IN_JSONFILE>"
    password:
      source: file
      file:
        path: <JSON_FILE>
        format: json
        jsonPath: "$.<KEY_FOR_PASSWORD_IN_JSONFILE>"
```
```
   mtls:
    cert:
      source: file
      file:
        path: <JSON_FILE>
        format: json
        jsonPath: "$.<KEY_FOR_CERT_IN_JSONFILE>"
    certKey:
      source: file
      file:
        path: <JSON_FILE>
        format: json
        jsonPath: "$.<KEY_FOR_KEY_IN_JSONFILE>"
    serverCa:
      source: file
      file:
        path: <JSON_FILE>
        format: json
        jsonPath: "$.<KEY_FOR_SERVERCA_IN_JSONFILE>"
```
All of the secrets for Basic Auth and mTLS are defined as `SourceRef` - see `common` package for more info.

### Event creation and sending

The core of the library is a `SendEvent` function that handles sending of the events and a set of `New...Event` functions that create the events and map them into `plog.Logs` - an OpenTelemetry logs type that's propagated through the collector's pipeline.

`New<EVENT_TYPE>Event(<ARGUMENTS_EXPECTED_BY_EVENT_TYPE>) (plog.Logs, error)`  
The set of functions present in this library take arguments based on desired event type and return a `plog.Logs` object or a possible validation error. See below for the full list of event types and their expected arguments.

`func SendEvent(ctx context.Context, auditCfg *common.AuditConfig, logs plog.Logs)`  
[plog.Logs](https://pkg.go.dev/go.opentelemetry.io/collector/pdata/plog@v1.26.0#Logs) The best way to utilize `SendEvent` is to use one of the event creating functions provided by the library to create the event and map it into `plog.Logs`. Consuming services should handle the possible validation error and if there's none pass the created `plog.Logs` object to `SendEvent` along with the context to be then marshalled and sent out to the collector.  

Example call on the consumer side would look like:
```
logs, err := NewWorkflowEvent("SomeID", "SomeValue") // or any event creating function
if err != nil {
  // handle validation error
}
SendEvent(ctx, &cfg.Audit, logs)
```

## Event catalog
| Event type             |                                                   Function signature                                                    |  
|------------------------|:-----------------------------------------------------------------------------------------------------------------------:|
| `keyCreate`            |              `NewKeyCreateEvent(objectID string, l KeyLevel, t KeyCreateActionType, value any, dpp bool)`               | 
| `keyDelete`            |                          `NewKeyDeleteEvent(objectID string, l KeyLevel, value any, dpp bool)`                          | 
| `keyUpdate`            | `NewKeyUpdateEvent(objectID, propertyName string, l KeyLevel, t KeyUpdateActionType, oldValue, newValue any, dpp bool)` | 
| `keyRead`              |    `NewKeyReadEvent(objectID, channelType, channelID string, l KeyLevel, t KeyReadActionType, value any, dpp bool)`     | 
| `workflowStart`        |                  `NewWorkflowStartEvent(objectID, channelID, channelType string, value any, dpp bool)`                  |      
| `workflowUpdate`       |                       `NewWorkflowUpdateEvent(objectID string, oldValue, newValue any, dpp bool)`                       |      
| `workflowExecute`      |                 `NewWorkflowExecuteEvent(objectID, channelID, channelType string, value any, dpp bool)`                 |      
| `workflowTerminate`    |                `NewWorkflowTerminateEvent(objectID, channelID, channelType string, value any, dpp bool)`                |      
| `groupCreate`          |                               `NewGroupCreateEvent(objectID string, value any, dpp bool)`                               |        
| `groupRead`            |                                `NewGroupReadEvent(objectID string, value any, dpp bool)`                                |        
| `groupDelete`          |                               `NewGroupDeleteEvent(objectID string, value any, dpp bool)`                               |        
| `groupUpdate`          |                 `NewGroupUpdateEvent(objectID, propertyName string, oldValue, newValue any, dpp bool)`                  |     
| `userLoginSuccess`     |              `NewUserLoginSuccessEvent(objectID string, l LoginMethod, t MfaType, u UserType, value any)`               |  
| `userLoginFailure`     |                   `NewUserLoginFailureEvent(objectID string, l LoginMethod, f FailReason, value any)`                   | 
| `tenantOnboarding`     |                                 `NewTenantOnboardingEvent(objectID string, value any)`                                  |
| `tenantOffboarding`    |                                 `NewTenantOffboardingEvent(objectID string, value any)`                                 | 
| `tenantUpdate`         |         `NewTenantUpdateEvent(objectID, propertyName string, t TenantUpdateActionType, oldValue, newValue any)`         | 
| `configurationCreate`  |                                `NewConfigurationCreateEvent(objectID string, value any)`                                | 
| `configurationRead`    |                `NewConfigurationReadEvent(objectID, channelType, channelID string, value any)`         `                |
| `configurationDelete`  |                                `NewConfigurationDeleteEvent(objectID string, value any)`                                |
| `configurationUpdate`  |                         `NewConfigurationUpdateEvent(objectID string, oldValue, newValue any)`                          |
| `credentialCreate`     |                      `NewCredentialCreateEvent(credentialID string, c CredentialType, value any)`                       |
| `credentialExpiration` |                    `NewCredentialExpirationEvent(credentialID string, c CredentialType, value any)`                     |
| `credentialDelete`     |                      `NewCredentialDeleteEvent(credentialID string, c CredentialType, value any)`                       |
| `credentialRevokation` |                    `NewCredentialRevokationEvent(credentialID string, c CredentialType, value any)`                     |

All the enums in the functions above are provided within this library. For `UserLoginSuccess` and `UserLoginFailure`, values for enum types can be empty (it will be set to `UNSPECIFIED`), but if they are provided they must match the enums defined in this library, otherwise there will be an error. For all others and for `objectID`, `channelID` and `channelType` properties, it is necessary to provide a valid (non-empty) value. Additionally for `configurationCreate`, `configurationRead`, `configurationDelete` and `configurationUpdate` `*value` properties are required.