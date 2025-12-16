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
To use the library, consuming service must first instantiate a new audit logger using the config data, create event object and call the sending function to dispatch event.

#### Creating events

To create the event use one of provided `New<EVENT_TYPE>Event(eventMetadata EventMetadata, args ...) (plog.Logs, error)` functions. For each type it expects a `EventMetadata` object - it contains fields shared across each event type. To create one, use `NewEventMetadata(userInitiatorID, tenantID, eventCorrelationID string)` (`userInitiatorID` and `tenantID` are mandatory).

#### Sending events

Created event should be passed to `SendEvent` function that takes care of dispatching the event to collector defined in the config. 

Full set of calls on the consumer side would look like for example like this:
```
auditLogger, _ := otlpaudit.NewLogger(&cfg.Audit)
eventMetadata, _ := otlpaudit.NewEventMetadata("userInitID", "tenantID", "eventCorrelationID")
event, _ := otlpaudit.NewCmkCreateEvent(eventMetadata, "cmkID")
auditLogger.SendEvent(ctx, event) 
```
#### Additional Properties

There is also a functionality of additional properties introduced that allow to add properties to OTLP logs separate from those belonging to specific event types. Please keep in mind that they'll be propagated to **every** event. The additional properties are loaded via config as a literal:
```
additionalProperties: |
    property1: x
    property2: y
```


## Event catalog
| Event type               |                                                       Function signature                                                        |  
|--------------------------|:-------------------------------------------------------------------------------------------------------------------------------:|
| `keyCreate`              |                    `NewKeyCreateEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                     | 
| `keyDelete`              |                    `NewKeyDeleteEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                     | 
| `keyRestore`             |                    `NewKeyRestoreEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                    | 
| `keyPurge`               |                     `NewKeyPurgeEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                     | 
| `keyRotate`              |                    `NewKeyRotateEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                     | 
| `keyEnable`              |                    `NewKeyEnableEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                     | 
| `keyDisable`             |                    `NewKeyDisableEvent(metadata EventMetadata, objectID, systemID, cmkID string, t KeyType)`                    | 
| `workflowStart`          |          `NewWorkflowStartEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool)`          |      
| `workflowUpdate`         |               `NewWorkflowUpdateEvent(metadata EventMetadata, objectID string, oldValue, newValue any, dpp bool)`               |      
| `workflowExecute`        |         `NewWorkflowExecuteEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool)`         |      
| `workflowTerminate`      |        `NewWorkflowTerminateEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool)`        |      
| `groupCreate`            |                       `NewGroupCreateEvent(metadata EventMetadata, objectID string, value any, dpp bool)`                       |        
| `groupRead`              |            `NewGroupReadEvent(metadata EventMetadata, objectID, channelID, channelType string, value any, dpp bool)`            |        
| `groupDelete`            |                       `NewGroupDeleteEvent(metadata EventMetadata, objectID string, value any, dpp bool)`                       |        
| `groupUpdate`            |         `NewGroupUpdateEvent(metadata EventMetadata, objectID, propertyName string, oldValue, newValue any, dpp bool)`          |     
| `userLoginSuccess`       |      `NewUserLoginSuccessEvent(metadata EventMetadata, objectID string, l LoginMethod, t MfaType, u UserType, value any)`       |  
| `userLoginFailure`       |           `NewUserLoginFailureEvent(metadata EventMetadata, objectID string, l LoginMethod, f FailReason, value any)`           | 
| `tenantOnboarding`       |                               `NewTenantOnboardingEvent(metadata EventMetadata, tenantID string)`                               |
| `tenantOffboarding`      |                              `NewTenantOffboardingEvent(metadata EventMetadata, tenantID string)`                               | 
| `tenantUpdate`           | `NewTenantUpdateEvent(metadata EventMetadata, objectID, propertyName string, t TenantUpdateActionType, oldValue, newValue any)` | 
| `configurationCreate`    |                        `NewConfigurationCreateEvent(metadata EventMetadata, objectID string, value any)`                        | 
| `configurationRead`      |            `NewConfigurationUpdateEvent(metadata EventMetadata, objectID string, oldValue, newValue any)`         `             |
| `configurationDelete`    |                        `NewConfigurationDeleteEvent(metadata EventMetadata, objectID string, value any)`                        |
| `configurationUpdate`    |             `NewConfigurationReadEvent(metadata EventMetadata, objectID, channelType, channelID string, value any)`             |
| `credentialCreate`       |                    `NewCredentialCreateEvent(metadata EventMetadata, credentialID string, c CredentialType)`                    |
| `credentialExpiration`   |                  `NewCredentialExpirationEvent(metadata EventMetadata, credentialID string, c CredentialType)`                  |
| `credentialDelete`       |                    `NewCredentialDeleteEvent(metadata EventMetadata, credentialID string, c CredentialType)`                    |
| `credentialRevokation`   |                  `NewCredentialRevokationEvent(metadata EventMetadata, credentialID string, c CredentialType)`                  |
| `cmkOnboarding`          |                             `NewCmkOnboardingEvent(metadata EventMetadata, cmkID, systemID string)`                             |
| `cmkOffboarding`         |                            `NewCmkOffboardingEvent(metadata EventMetadata, cmkID, systemID string)`                             |
| `cmkSwitch`              |                          `NewCmkSwitchEvent(metadata EventMetadata, cmkID, cmkIDOld, cmkIDNew string)`                          |
| `cmkTenantModification`  |                  `NewCmkTenantModificationEvent(metadata EventMetadata, cmkID, systemID string, c CmkAction)`                   |
| `cmkTenantDelete`        |                                 `NewCmkTenantDeleteEvent(metadata EventMetadata, cmkID string)`                                 |
| `cmkCreate`              |                                    `NewCmkCreateEvent(metadata EventMetadata, cmkID string)`                                    |
| `cmkDelete`              |                                    `NewCmkDeleteEvent(metadata EventMetadata, cmkID string)`                                    |
| `cmkDetach`              |                           `NewCmkDetachEvent(metadata EventMetadata, cmkID string, systemID string)`                            |
| `cmkRestore`             |                                   `NewCmkRestoreEvent(metadata EventMetadata, cmkID string)`                                    |
| `cmkEnable`              |                                    `NewCmkEnableEvent(metadata EventMetadata, cmkID string)`                                    |
| `cmkDisable`             |                                   `NewCmkDisableEvent(metadata EventMetadata, cmkID string)`                                    |
| `cmkRotate`              |                                    `NewCmkRotateEvent(metadata EventMetadata, cmkID string)`                                    |
| `cmkAvailable`           |                                  `NewCmkAvailableEvent(metadata EventMetadata, cmkID string)`                                   |
| `cmkUnavailable`         |                                 `NewCmkUnavailableEvent(metadata EventMetadata, cmkID string)`                                  |
| `unauthenticatedRequest` |                                    `NewUnauthenticatedRequestEvent(metadata EventMetadata)`                                     |
| `unauthorizedRequest`    |                      `NewUnauthorizedRequestEvent(metadata EventMetadata, resource string, action string)`                      |

All the enums in the functions above are provided within this library. For every enum values can be empty (it will be set to `UNSPECIFIED`), but if they are provided they must match the enums defined in this library, otherwise there will be an error. All `*value` properties are optional with the exception of ones present in event types: `tenantUpdate`, `configurationCreate`, `configurationRead`, `configurationDelete` and `configurationUpdate`. All other properties are considered required.