# Design

## Workflows

### Initial User Registration

The user must register with the Station Manager online service before they can access any online service. The registration
process is as follows:

1. User enters their *username*, *email address*, and a *password*.
2. Station Manager sends an email to the user with a link to confirm their email address.
3. User clicks the link in the email to confirm their email address.
4. When the user confirms their email address, a *bootstrap token* is generated and displayed to the user. This token
is time-limited (valid for 24 hours) and can only be used once. The bootstrap token is a one-time
authentication token for the creation of the account's **first** logbook.

### First Logbook Creation

The user must create their first logbook using the bootstrap token, and they received from Station Manager, and within
the 24-hour validity period of the token. The bootstrap token is used to authenticate the user when they access the
Station Manager online service to create the account's **first** logbook.

1. User creates a new logbook and enters the bootstrap token into the appropriate field.
2. 