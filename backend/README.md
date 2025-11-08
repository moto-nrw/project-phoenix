# Go Restful API Boilerplate

[![GoDoc Badge]][godoc] [![GoReportCard Badge]][goreportcard]

Easily extendible RESTful API boilerplate aiming to follow idiomatic go and best
practice.

The goal of this boiler is to have a solid and structured foundation to build
upon on.

Any feedback and pull requests are welcome and highly appreciated. Feel free to
open issues just for comments and discussions.

## Features

The following feature set is a minimal selection of typical Web API
requirements:

- Configuration using [viper](https://github.com/spf13/viper)
- CLI features using [cobra](https://github.com/spf13/cobra)
- PostgreSQL support including migrations using
  [bun](https://github.com/uptrace/bun)
- Structured logging with [Logrus](https://github.com/sirupsen/logrus)
- Routing with [chi router](https://github.com/go-chi/chi) and middleware,
  following
  [chi rest example](https://github.com/go-chi/chi/tree/master/_examples/rest)
- JWT Authentication using [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx)
  with username/password authentication
- Request data validation using
  [ozzo-validation](https://github.com/go-ozzo/ozzo-validation)
- HTML emails with [go-mail](https://github.com/go-mail/mail)

## Start Application

- Clone and change into this repository

### Local

- Create a postgres database and adjust environment variables in dev.env
- Run the application to see available commands: `go run main.go`
- Run all migrations from database/migrate folder: `go run main.go migrate`
- Run the application with command _serve_: `go run main.go serve`

### Using Docker Compose

- First start the database only: `docker compose up -d postgres`
- Once initialize the database by running all migrations in database/migrate
  folder: `docker compose run server ./main migrate`
- Start the api server: `docker compose up`

### Environment Variables

By default viper will look at dev.env for a config file. It contains the
applications defaults if no environment variables are set otherwise.

## API Routes

### Authentication

The application uses username/password authentication with the following routes:

| Path                  | Method | Required JSON                                    | Header                                | Description                             |
| --------------------- | ------ | ------------------------------------------------ | ------------------------------------- | --------------------------------------- |
| /auth/login           | POST   | email, password                                  |                                       | Login with email and password           |
| /auth/register        | POST   | email, name, password, confirm_password          |                                       | Register a new account                  |
| /auth/change-password | POST   | current_password, new_password, confirm_password | Authorization: "Bearer access_token"  | Change password (must be authenticated) |
| /auth/refresh         | POST   |                                                  | Authorization: "Bearer refresh_token" | Refresh JWTs                            |
| /auth/logout          | POST   |                                                  | Authorization: "Bearer refresh_token" | Logout from this device                 |

Passwords must meet complexity requirements: minimum 8 characters, uppercase,
lowercase, numbers, and special characters.

### Example API

The example api follows the patterns from the
[chi rest example](https://github.com/go-chi/chi/tree/master/_examples/rest).
Besides _/auth_ routes the API provides two main routes for _/api_ and _/admin_
requests, the latter requires to be logged in as administrator by providing the
respective JWT in Authorization Header.

Check [routes.md](routes.md) for a generated overview of the provided API
routes.

### Testing

Package auth/userpass contains example api tests using a mocked database. Run
them with: `go test -v ./...`

### Client API Access and CORS

The server is configured to serve a Progressive Web App (PWA) client from
_./public_ folder (this repo only serves an example index.html, see below for a
demo PWA client to put here). In this case enabling CORS is not required,
because the client is served from the same host as the api.

If you want to access the api from a client that is served from a different
host, including e.g. a development live reloading server with below demo client,
you must enable CORS on the server first by setting environment variable
_ENABLE_CORS=true_ on the server to allow api connections from clients served by
other hosts.

#### Demo client application

A deployed version can also be found at
[go-base.onrender.com](https://go-base.onrender.com) (takes up to 60 seconds to
spin up if sleeping...)

For demonstration of the login and account management features this API serves a
demo [Vue.js](https://vuejs.org) PWA. The client's source code can be found
[here](https://github.com/moto-nrw/project-phoenix-vue). Build and put it into
the api's _./public_ folder, or use the live development server (requires
ENABLE_CORS environment variable set to true).

Use one of the following bootstrapped users for login:

- <admin@example.com> (has access to admin panel)
- <user@example.com>

[godoc]: https://godoc.org/github.com/moto-nrw/project-phoenix
[godoc badge]: https://godoc.org/github.com/moto-nrw/project-phoenix?status.svg
[goreportcard]:
  https://goreportcard.com/report/github.com/moto-nrw/project-phoenix
[goreportcard badge]:
  https://goreportcard.com/badge/github.com/moto-nrw/project-phoenix
