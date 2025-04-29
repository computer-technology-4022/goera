# Goera - Online Code Judge System

Goera is a distributed online code judge system that allows users to submit code solutions and get them evaluated in real-time. The system consists of multiple microservices working together to provide a seamless code evaluation experience.

## Architecture

The system is composed of the following main components:

- **Judge Service**: Handles code evaluation and scoring
- **Code Runner**: Executes submitted code in a secure environment
- **Serve Service**: Main API service that handles user requests
- **PostgreSQL Database**: Stores user data and submission information

## Prerequisites

- Docker and Docker Compose
- Go 1.21 or later (for development)

## Getting Started

1. Clone the repository:

```bash
git clone <repository-url>
cd goera
```

2. Start the services using Docker Compose:

```bash
docker-compose up --build
```

The services will be available at ports:

- Judge API: :8080
- Main API: :5000
- Database: :5433 (PostgreSQL)

## Project Structure

```
goera/
├── judge/           # Judge service and code runner
├── serve/           # Main API service
├── code-runner/     # Code execution environment
├── docker-compose.yaml
└── README.md
```

## Development

### Building Services

Each service has its own Dockerfile and can be built independently:

```bash
# Build judge service
docker-compose build judge

# Build serve service
docker-compose build serve
```

### Environment Variables

The services use the following environment variables:

**Judge Service:**

- `JUDGE_API_URL`: URL of the judge API
- `INTERNAL_API_KEY`: API key for internal communication

**Serve Service:**

- `PORT`: Service port (default: 5000)
- `JUDGE_API_URL`: URL of the judge API
- `DB_HOST`: Database host
- `DB_PORT`: Database port
- `DB_USER`: Database username
- `DB_PASSWORD`: Database password
- `DB_NAME`: Database name
- `DB_SSLMODE`: Database SSL mode

## Database

The system uses PostgreSQL as its database. The database is configured with the following defaults:

- Username: postgres
- Password: example
- Database: app
- Port: 5433 (mapped from container port 5432)

## Security Notes

- The system uses privileged containers for code execution. This is necessary for the code runner but should be used with caution.
- In production, sensitive information like database passwords and API keys should be managed using Docker secrets or environment variables.
- The database connection uses SSL mode disabled by default. For production, enable SSL and use proper certificates.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

