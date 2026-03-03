# Sample Custom Resources

This directory contains sample User and Model custom resources for testing and demonstration purposes.

## Files

- **user_alice.yaml** - Sample user with `systems_role` attribute
- **user_bob.yaml** - Sample user with `database_expert` attribute
- **models.yaml** - Sample base model and LoRA adapters with access policies

## Usage

### Apply all samples

```bash
kubectl apply -f config/samples/
```

### Apply individual samples

```bash
kubectl apply -f config/samples/user_alice.yaml
kubectl apply -f config/samples/user_bob.yaml
kubectl apply -f config/samples/models.yaml
```

## Sample Policies

### Users

**Alice** (systems_role):

- Has access to: base model, z17-technical LoRA, power-env LoRA
- Does NOT have access to: storage-flash LoRA

**Bob** (database_expert):

- Has access to: base model, storage-flash LoRA
- Does NOT have access to: z17-technical LoRA, power-env LoRA

### Models

**Base Model**: Llama-3.2-1B-Instruct

- Accessible by: secret_role, public_role, systems_role, database_expert

**LoRA Adapters**:

1. **ibm-z17-technical** - IBM Z17 system expertise
   - Access: systems_role only
   - Selection keywords: IBM z17, z systems

2. **ibm-power-env** - IBM Power Systems with Ansible
   - Access: systems_role only
   - Selection keywords: ansible, IBM power system

3. **ibm-storage-flash** - IBM Flash Storage database management
   - Access: database_expert only
   - Selection keywords: flash system, database management

## Testing Access Control

After applying these samples, you can test access control by:

1. Generate JWT tokens for alice and bob:

   ```bash
   ./bin/amd64/llmd-auth create --name alice
   ./bin/amd64/llmd-auth create --name bob
   ```

2. Test API calls with the generated tokens:

   ```bash
   # Alice can access z17 LoRA
   curl -H "Authorization: Bearer <alice-token>" \
        -H "ext-proc-enable: allow" \
        -d '{"model":"meta-llama/Llama-3.2-1B-Instruct","prompt":"Tell me about z17"}' \
        https://<gateway>/v1/completions

   # Bob can access storage LoRA
   curl -H "Authorization: Bearer <bob-token>" \
        -H "ext-proc-enable: allow" \
        -d '{"model":"meta-llama/Llama-3.2-1B-Instruct","prompt":"Tell me about flash storage"}' \
        https://<gateway>/v1/completions
   ```

3. List accessible models:

   ```bash
   # Alice's view
   curl -H "Authorization: Bearer <alice-token>" \
        https://<gateway>/v1/models

   # Bob's view
   curl -H "Authorization: Bearer <bob-token>" \
        https://<gateway>/v1/models
   ```
