## ADDED Requirements

### Requirement: Plan Definition

Administrators SHALL be able to define purchasable plans.

#### Scenario: Create a plan

- **WHEN** an admin creates a plan with name, price, traffic allowance (GB),
  duration (days), optional IP limit, and an enabled flag
- **THEN** the plan is persisted and becomes available for purchase

#### Scenario: Disable a plan

- **WHEN** an admin disables a plan
- **THEN** the plan SHALL remain valid for clients already provisioned from it
  but SHALL NOT be offered for new purchases

### Requirement: User Balance

The system SHALL maintain a monetary balance per dashboard user.

#### Scenario: Balance starts at zero

- **WHEN** a new dashboard user account is created
- **THEN** its balance SHALL be zero

#### Scenario: Admin adjusts balance

- **WHEN** an admin credits or debits a user's balance
- **THEN** the new balance is persisted and a balance-history entry recording the
  amount, reason, and actor SHALL be created

### Requirement: Plan Purchase

Users SHALL be able to purchase a plan, which provisions or extends a client.

#### Scenario: Successful purchase

- **WHEN** an authenticated `user` purchases an enabled plan and their balance
  covers the price
- **THEN** the system deducts the price, creates an order record, and invokes
  client provisioning to create or extend the user's client per the plan's
  traffic and duration

#### Scenario: Insufficient balance

- **WHEN** a user purchases a plan their balance cannot cover
- **THEN** the purchase is rejected, no order is created, and no client change occurs

#### Scenario: Provisioning failure rolls back

- **WHEN** the balance is deducted but client provisioning on the node fails
- **THEN** the system SHALL refund the deducted amount and mark the order failed,
  leaving the balance consistent

### Requirement: Order History

The system SHALL record every purchase as an order.

#### Scenario: User views own orders

- **WHEN** an authenticated `user` opens their order history
- **THEN** the system returns that user's orders with plan name, amount, status,
  and timestamp

#### Scenario: Admin views all orders

- **WHEN** an admin opens order management
- **THEN** the system returns a paginated, filterable list of all orders across users

### Requirement: Idempotent Purchase Submission

The system SHALL prevent a double-click or retried purchase from charging twice.

#### Scenario: Duplicate purchase request

- **WHEN** the same purchase request (same idempotency key) is submitted more than once
- **THEN** the system SHALL process it at most once and return the original order
  for subsequent submissions
