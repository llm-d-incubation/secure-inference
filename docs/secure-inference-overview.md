# Your LLM Queries to Confidential Data Isn’t Private—Even If It Runs in a ‘Secure’ Container

**By**: Dasari Surya Sai Venkatesh, Pavani Penumalla, Pravein K. Govindan, Arun Kumar at IBM Research India | **Reading Time**: 8 minutes | **Published**: April 2026

---

## The Million Dollar Question Every Enterprise Leader Faces

The LLMs today don't know who's asking the query and what content are they allowed to consume or not.

Imagine that you've invested millions in AI infrastructure. Your teams are deploying Large Language Models for code assistance, document analysis, financial forecasting, and customer intelligence. Productivity is soaring. Innovation is accelerating.

Then your CISO asks: *"How do we prevent our AI from leaking confidential information to unauthorized users?"*

Suddenly, you realize the problem isn't just about simple and plain access control, it is about **context-aware intelligence**. The same question about next quarter's product roadmap should yield radically different answers depending on whether the requester is an executive, a contractor, or a partner. Your HR assistant knows salary data and reorganization plans, but shouldn't share everything with everyone. Engineers fine-tuning models on proprietary code risk embedding trade secrets into model weights themselves.


---

## Why Traditional Approaches Fail

Most enterprises handle confidential AI data through one of three inadequate strategies:

**AI Lockdown**: Ban all confidential data from LLMs. **Result**: You lose 60-80% of potential AI value. Impossible to enforce; frequently violated in shadow IT deployments.

**Private Silos with Binary Access**: Deploy isolated LLM instances per department, so the users either have full access to an LLM instance or none. **Result**: Infrastructure costs explode after a few teams. Annual spend can run into millions for infrastructure alone, plus operational burden of managing multiple systems.

**Post-Hoc Filtering**: Deploy guardrails that scan outputs for sensitive content. **Result**: False negatives could create compliance exposure while false positives end up destroying user experience.

### The Root Cause

When you **fine-tune a model on confidential data**, that knowledge embeds into model weights. Now,current LLMs don't distinguish between requesters. The model treats all authenticated users identically whereas:
- A peer asking about a colleague's project should see timeline and basic scope
- An HR partner asking the same question should see compensation, performance notes, and full details
- A contractor asking the same question should see limited public information only

The common 'binary access' solution adopted today, i.e. users get nothing or everything, creates operational gridlock and compliance risk. Organizations are forced to choose between AI productivity and data security. They shouldn't have to.

---

## The Solution: Adapter-Based Access Control

**Secure Inference** solves this through a fundamentally different architecture: instead of deploying separate LLMs for each security domain, it uses **LoRA adapters** (Low-Rank Adaptation—lightweight model extensions) with **policy-enforced routing**.

### How It Works

**Training Phase**: Partition sensitive data by security domain. Create small LoRA adapters for each:
- **Financial LoRA**: Earnings data, forecasts, M&A plans 
- **HR LoRA**: Compensation, reviews, org changes
- **Product LoRA**: Roadmaps, customer data
- **Engineering LoRA**: Proprietary code, architecture

Each adapter loads quickly and contains only its security domain's knowledge.

**Inference Phase**: When a user makes a request:
1. **Authenticate**: Verify identity via JWT token or SSO
2. **Authorize**: Check user attributes (role, department, clearance level) against policy engine (Open Policy Agent)
3. **Select Adapter**: Match query content to appropriate adapter(s) based on semantic analysis
4. **Route**: Send request to inference pool with correct adapter loaded
5. **Enforce**: Users without authorization receive responses from base model only (general knowledge), never from confidential adapters

**Critical Security Property**: Authorization happens at the gateway, before the request reaches the model. Unauthorized users cannot access restricted adapters under any circumstances.

### Example Flow

```
User: "Show me database optimization code for the customer service platform"
        ↓
Gateway authenticates JWT (role: engineer, dept: platform, clearance: internal)
        ↓
Policy Check: Does user have access to Engineering LoRA? → YES
        ↓
Semantic Matching: Best adapter = Engineering LoRA (similarity: 0.91)
        ↓
Route to inference pool with Engineering LoRA loaded
        ↓
Response: Detailed code snippets from proprietary codebase with architecture context
```

A contractor with the same query would be denied access to Engineering LoRA and receive only general programming advice from the base model.

---

## Usecase Scenarios

### Financial Services: Protecting Market-Moving Intelligence

**Organization**: Global investment bank

**Challenge**: Research analysts produce proprietary earnings models and M&A intelligence. Traders need AI assistance but shouldn't see pre-publication research. Compliance requires proving no insider information leaked to unauthorized parties.

**Implementation**:
- Public Markets LoRA (accessible to all traders)
- Proprietary Research LoRA (research team + executives only)
- M&A LoRA (investment banking + legal only)

**Estimated Gains**:
- Productivity increase for research analysts with proprietary research LoRA based inferencing
- Unauthorized access prevented
- Automatic audit trail demonstrates regulatory compliance

**Key Metric**: Analysts now get AI assistance on proprietary models securely and without the additional cost of dedicated infrastructure

---

### Healthcare: HIPAA-Compliant Clinical AI

**Organization**: Hospital system

**Challenge**: Clinical decision support needs patient records (PHI), but nurses, physicians, and specialists have different access rights. HIPAA requires encryption, access logging, and minimum necessary access enforcement.

**Implementation**:
- General Medical LoRA (all clinicians: medical literature, drug interactions)
- Patient PHI LoRA (treating physicians only, scoped to assigned patients)
- Research LoRA (approved researchers: de-identified aggregated data)

**Estimated Gains**:
- Reduction in diagnostic time via AI-assisted clinical reasoning
- Audit trail proves minimum necessary access
- No manual access control management; policy-driven automation

**Key Metric**: Prevention of PHI exposure incidents while enabling AI assisted reasoning

---

### Technology Company: Protecting Intellectual Property

**Organization**: SaaS company

**Challenge**: Engineers need AI code assistance on proprietary codebase. Contractors shouldn't access core platform code. Product managers need customer insights but shouldn't see source code.

**Implementation**:
- Public Code LoRA (all engineers: open-source libraries, common patterns)
- Proprietary Platform LoRA (full-time engineers with core access only)
- Customer Insights LoRA (product managers + customer success: analytics, feedback)

**Estimated Gains**:
- Faster code review cycle using internal best practices
- No IP leakage incidents as contractors are isolated from proprietary code
- Custom model(s) trained on proprietary codebase

**Key Metric**: Engineers now get context-aware suggestions from the actual codebase, not generic Stack Overflow patterns.

---

## Technical Foundation: Built on llm-d

Secure Inference integrates with **llm-d**, an open-source Kubernetes-native distributed LLM inference framework that provides:
- **Intelligent Scheduling**: Auto-loads appropriate adapters based on request patterns
- **Cache-Aware Routing**: Routes similar queries to pods with adapters already loaded (reduces latency)
- **Disaggregated Serving**: Separates prefill from decode for efficiency
- **Scalable Multi-Node**: Supports large number of adapters across distributed infrastructure

**Why Kubernetes-Native Matters**: Platform teams already know Kubernetes and no new skills required. Runs on public cloud platforms or on-premises. Standard tooling (Helm, Prometheus, kubectl). Zero vendor lock-in (Apache 2.0 license).

**Architecture**:
1. **Training Phase**: Sensitive data → Partitioned by domain → Train LoRA adapters → Store in access-controlled registry
2. **Inference Phase**: User request → Gateway (Auth + Policy) → Adapter Selection → Route to Inference Pool → Generate Response → Audit Log

---

## Getting Started
**Quick Start**: Review the [5-minute quickstart guide](https://github.com/llm-d-incubation/secure-inference#quick-start)  
**GitHub Repository**: [github.com/llm-d-incubation/secure-inference](https://github.com/llm-d-incubation/secure-inference). Star the repository to follow development progress or join the community to contribute  
**License**: Apache 2.0 (open source, production-ready)


---

## The Bottom Line

The enterprise AI security paradox is solvable. You don't have to choose between AI productivity and data security, innovation velocity and compliance, or centralized efficiency and departmental autonomy.

Secure Inference proves you can have all three—by making access control a **first-class concern at inference time**, not an afterthought.

**The question isn't whether your organization will adopt confidential AI. The question is whether you'll do it securely.**

---

*LLM-D is an open-source initiative within CNCF foundation, building production-grade infrastructure for enterprise AI deployments. Secure Inference addresses critical security and access control requirements for organizations running AI at scale.*

