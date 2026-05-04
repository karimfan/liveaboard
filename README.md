We are building a multi-tenant, cloud-based SaaS platform designed specifically for scuba diving liveaboards, which are boats that function as floating hospitality and diving operations. Each operator (organization) manages one or more boats, and each boat runs discrete trips where guests live onboard for a defined period of time, typically ranging from one week to one month. The system serves as the operational backbone for site directors and crew, enabling them to manage trips, track guests and crew, and capture all onboard economic activity in a structured and reliable way.

The core abstraction of the system is the trip. A trip represents a bounded operational window for a specific boat, with a defined start and end date, a manifest of guests, and a roster of crew and site directors responsible for running operations. Trips are independent from one another but inherit structural configuration from the boat, such as overall guest capacity. The system must support the creation and management of organizations, boats, and trips, with appropriate relationships and constraints, such as preventing overbooking and ensuring each trip has designated operational leadership.

Within each trip, the platform must act as a real-time ledger of guest activity and consumption. Guests consume a wide range of items and services during a trip, including equipment rentals, merchandise, food and beverage (including alcohol), and additional services like laundry or guided dives. These offerings vary significantly by operator and boat, so the system must provide a flexible, configurable catalog that allows each organization to define its own items, pricing models, and inventory rules. All consumption is recorded against individual guests as ledger entries, forming a continuously updated account of what each guest has used and what they owe.

## Personas

The system serves the following user personas:

- **Organization Admin**: Manages org-wide operations including creating trips, managing the fleet (adding/removing boats), and configuring the item catalog and pricing. Also has org-wide visibility into reporting, analytics, and financial oversight across all boats and trips.
- **Site Director**: Responsible for running a single trip. Manages the guest manifest, records guest consumption, and oversees all onboard operations for the duration of the trip.
- **Guest** (future): Self-service view of their own running tab, dive schedule, and trip details. Low priority but accounted for in the data model.

## Operational Environment

The system must be designed for real-world operating conditions on liveaboards, including fast-paced environments and multiple crew members interacting with the system simultaneously. The system assumes online connectivity by default. The user experience must prioritize speed and simplicity, enabling crew to quickly record transactions and manage trip data with minimal friction. Over time, the platform should also support reporting and analytics across trips, boats, and organizations, providing insights into revenue, inventory usage, and guest behavior, while maintaining strict data isolation between organizations.

