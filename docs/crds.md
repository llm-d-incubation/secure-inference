# User and Model CRDs

secure-inference uses two Kubernetes custom resources to define access control policies.

**API group:** `accesscontrol.llm-d.io/v1alpha1`

## User

A User represents an identity that can make inference requests.

```yaml
apiVersion: accesscontrol.llm-d.io/v1alpha1
kind: User
metadata:
  name: alice
spec:
  id: alice
  attributes:
    role: systems_role
```

| Field | Required | Description |
|---|---|---|
| `spec.id` | yes | Unique identifier, must match the `username` claim in the JWT |
| `spec.attributes` | yes | Key-value pairs used for access policy matching |

## Model

A Model represents either a base model or a LoRA adapter that users can request.

### Base Model

```yaml
apiVersion: accesscontrol.llm-d.io/v1alpha1
kind: Model
metadata:
  name: llama-1b-base
spec:
  id: "meta-llama/Llama-3.2-1B-Instruct"
  type: BaseModel
  accessPolicy:
    userAttributes:
      role:
        - systems_role
        - database_expert
```

### LoRA Adapter

```yaml
apiVersion: accesscontrol.llm-d.io/v1alpha1
kind: Model
metadata:
  name: ibm-z17-technical
spec:
  id: ibm_z17_technical_technical_introduction
  type: LoRA
  baseModelId: "meta-llama/Llama-3.2-1B-Instruct"
  accessPolicy:
    userAttributes:
      role:
        - systems_role
  selectionPolicy:
    descriptions:
      - "IBM z17 mainframe processor architecture and hardware specifications"
      - "IBM z Systems z17 technical introduction and system design"
```

| Field | Required | Description |
|---|---|---|
| `spec.id` | yes | Model identifier, must match the `model` field in requests |
| `spec.type` | yes | `BaseModel` or `LoRA` |
| `spec.baseModelId` | LoRA only | The base model this adapter extends |
| `spec.accessPolicy.userAttributes` | yes | Map of attribute keys to allowed values |
| `spec.selectionPolicy.descriptions` | no | Descriptions for semantic adapter selection |

## Access Policy

Access is evaluated using attribute-based access control (ABAC):

- For **each** attribute key in the model's `accessPolicy.userAttributes`, the user must have a matching value
- The user's value must match **at least one** of the allowed values for that key (OR within an attribute)
- **All** attribute keys must pass (AND across attributes)

Example:

```
Model accessPolicy:
  role: [systems_role, database_expert]    # user.role must be one of these
  department: [engineering]                # AND user.department must be one of these

User attributes:
  role: systems_role        -> matches "systems_role"
  department: engineering   -> matches "engineering"

Result: ALLOWED
```

## Sample Policies

See [config/samples/](../config/samples/) for complete working examples.
