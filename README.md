# HRMIS API - Human Resource Management Information System

A robust, production-ready HRMIS backend API built with Go, Fiber, PostgreSQL, Redis, and MinIO. This system manages personnel, attendance, leaves, payroll, and comprehensive HR operations.

## 🚀 Recent Improvements & Best Practices

### Code Quality Enhancements

#### ✅ **Enhanced Error Handling**
- Comprehensive AppError system with proper HTTP status codes
- Consistent error responses across all handlers
- Error type checkers (IsNotFound, IsValidation, IsUnauthorized, etc.)
- Proper error wrapping with context

#### ✅ **Handler Layer Improvements**
- Reduced context timeout from 30s to 10s for better performance
- Consistent use of utility functions (SuccessResponse, ErrorResponse, HandleError)
- Better input validation with descriptive error messages
- Eliminated duplicate error handling code
- Uses Fiber's context (`c.Context()`) instead of `context.Background()`

#### ✅ **Service Layer Enhancements**
- Proper use of custom AppError types
- Business logic validation with clear error messages
- Cleaner code with reusable helper functions
- Proper date/time parsing with utility functions

#### ✅ **Repository Pattern Improvements**
- Consistent error handling with AppError
- Proper GORM error checking (`gorm.ErrRecordNotFound`)
- Better error messages for database operations
- Context propagation throughout

#### ✅ **Utilities & Helpers**
- Response utility functions
- Centralized pagination logic
- Date/Time parsing utilities
- Pointer helpers

#### ✅ **Docker Optimization**
- Multi-stage builds (~15MB final image)
- Non-root user for security
- Health checks
- Optimized layer caching
- Comprehensive .dockerignore

## 📋 Prerequisites

- Go 1.25+ 
- Docker & Docker Compose (recommended)
- PostgreSQL 16+
- Redis 7+
- MinIO or AWS S3

## 🛠️ Installation

### Using Docker (Recommended)

```bash
# Start all services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

Services available at:
- **API**: http://localhost:4000
- **Swagger UI**: http://localhost:4000/swagger/index.html
- **MinIO Console**: http://localhost:9001

### Local Development

```bash
# Install dependencies
make deps

# Copy environment file
cp envs/.env.example envs/.env

# Edit configuration
nano envs/.env

# Run the application
make run
```

## 🔧 Configuration

Create `.env` in `envs/` directory:

```env
# Application
APP_PORT=4000
APP_ENV=development

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=hrmis
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10

# S3/MinIO
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin
AWS_DEFAULT_REGION=us-east-1
AWS_ENDPOINT=http://localhost:9000
AWS_USE_PATH_STYLE_ENDPOINT=true
AWS_BUCKET=hrmis

# JWT
JWT_SECRET=your-secret-key-change-in-production
JWT_EXPIRATION=24h

# CORS
CORS_ORIGINS=*
```

## 📚 API Documentation

Access Swagger UI at: http://localhost:4000/swagger/index.html

Regenerate docs:
```bash
make swagger
```

## 🔌 API Endpoints

### Authentication
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/register` - User registration

### Personnel Management
- `GET /api/v1/personnels` - List all personnel
- `GET /api/v1/personnels/:id` - Get personnel details
- `POST /api/v1/personnels` - Create personnel
- `PUT /api/v1/personnels/:id` - Update personnel
- `DELETE /api/v1/personnels/:id` - Delete personnel

### Attendance
- `GET /api/v1/attendances` - List attendances
- `POST /api/v1/attendances` - Create attendance
- `GET /api/v1/attendance-corrections` - List corrections
- `POST /api/v1/attendance-corrections` - Request correction

### Leave Management
- `GET /api/v1/leaves` - List leave requests
- `POST /api/v1/leaves` - Submit leave request
- `PUT /api/v1/leaves/:id` - Update leave request

### Payroll
- `GET /api/v1/payrolls` - List payrolls
- `POST /api/v1/payrolls` - Generate payroll
- `GET /api/v1/salary-components` - List salary components

### Health Check
- `GET /health` - System health status

## 🔐 Authentication

Use JWT tokens in the Authorization header:

```bash
Authorization: Bearer <your-jwt-token>
```

Example login:
```bash
curl -X POST http://localhost:4000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "password123"
  }'
```

## 📂 Project Structure

```
.
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration
│   ├── constants/              # Constants
│   ├── errors/                 # Custom errors
│   ├── handlers/               # HTTP handlers
│   ├── middleware/             # Middleware
│   ├── model/                  # Data models
│   ├── repository/             # Data access
│   ├── routes/                 # Routes
│   └── services/               # Business logic
├── pkg/
│   ├── jwt/                    # JWT utilities
│   └── utils/                  # Helpers
├── docs/                       # Swagger docs
├── docker/                     # Docker files
├── Makefile                    # Dev commands
└── docker-compose.yml          # Docker compose
```

## 🧪 Testing

```bash
# Run tests
make test

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...

# View coverage
go tool cover -html=coverage.out
```

## 🛠️ Makefile Commands

```bash
make help           # Show all commands
make run            # Run application
make build          # Build binary
make test           # Run tests
make docker-up      # Start docker services
make docker-down    # Stop docker services
make docker-logs    # View docker logs
make swagger        # Generate swagger docs
make clean          # Clean artifacts
make fmt            # Format code
make lint           # Run linter
```

## 🚢 Deployment

### Docker

```bash
# Build image
make docker-build

# Push to registry
docker tag hrmis-api:latest your-registry/hrmis-api:latest
docker push your-registry/hrmis-api:latest

# Deploy
docker-compose up -d
```

### Binary

```bash
# Build
make build

# Run
./bin/api-hrmis
```

## 📈 Performance Features

- Database connection pooling (25 max connections)
- Redis caching with automatic TTL
- Request timeout middleware (10s default)
- Optimized Docker image (~15MB)
- Context-aware operations
- Prepared statements via GORM

## 🔒 Security Features

- JWT authentication
- Non-root Docker user
- CORS configuration
- Input validation
- Prepared statements
- Environment-based secrets

## 📊 Health Check

```bash
curl http://localhost:4000/health
```

Response:
```json
{
  "status": "healthy",
  "services": {
    "database": "connected",
    "redis": "connected"
  }
}
```

## 📦 Key Dependencies

- **Fiber v2** - HTTP framework
- **GORM** - ORM
- **PostgreSQL** - Database
- **Redis** - Cache
- **MinIO/S3** - File storage
- **JWT** - Authentication
- **Swagger** - Documentation

## 🤝 Contributing

1. Fork the repository
2. Create feature branch
3. Make changes
4. Run tests: `make test`
5. Submit pull request

## 📄 License

[Your License]

## 👥 Support

- GitHub Issues
- Email: support@hrmis.com

## 📚 Resources

- [Go Docs](https://golang.org/doc/)
- [Fiber Docs](https://docs.gofiber.io/)
- [GORM Docs](https://gorm.io/)
- [PostgreSQL](https://www.postgresql.org/docs/)
- [Redis](https://redis.io/documentation)

DB_PASSWORD=your_password
DB_NAME=ganipedia
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5

# JWT
JWT_SECRET=your-secret-key-change-this-in-production

# Redis (Optional)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=10

# RabbitMQ (Optional)
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672

# S3/MinIO
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
AWS_DEFAULT_REGION=us-east-1
AWS_ENDPOINT=http://localhost:9000
AWS_USE_PATH_STYLE_ENDPOINT=true
AWS_BUCKET=ganipedia

# CORS
CORS_ORIGINS=*
```

4. **Run database migrations**

Migrations run automatically on application start.

## 🚀 Running the Application

### Development Mode
```bash
go run cmd/main.go
```

### Production Build
```bash
go build -o ganipedia-api cmd/main.go
./ganipedia-api
```

### Using Docker
```bash
docker build -f docker/Dockerfile -t ganipedia-api .
docker run -p 4000:4000 --env-file envs/.env ganipedia-api
```

The API will be available at `http://localhost:4000`

## 📚 API Documentation

Once the application is running, access the Swagger UI documentation at:

```
http://localhost:4000/swagger/index.html
```

### Regenerating Swagger Documentation

If you make changes to API comments:

```bash
# Install swag if not already installed
go install github.com/swaggo/swag/cmd/swag@latest

# Generate swagger docs
swag init -g cmd/main.go -o docs
```

## 🔌 API Endpoints

### Authentication
- `POST /api/v1/auth/register` - Register a new user
- `POST /api/v1/auth/login` - Login user

### Products (Public)
- `GET /api/v1/products` - Get all products (paginated)
- `GET /api/v1/products/:id` - Get product by ID

### Products (Protected - requires authentication)
- `POST /api/v1/products` - Create a new product
- `POST /api/v1/products/upload` - Upload product image
- `PUT /api/v1/products/:id` - Update product
- `DELETE /api/v1/products/:id` - Delete product

### Users (Protected - requires authentication)
- `GET /api/v1/users` - Get all users (paginated)
- `GET /api/v1/users/:id` - Get user by ID
- `POST /api/v1/users` - Create a new user
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user

### Health Check
- `GET /health` - Check API and dependencies health status

## 🔐 Authentication

The API uses JWT (JSON Web Tokens) for authentication. Include the token in the Authorization header:

```
Authorization: Bearer <your-jwt-token>
```

### Example Login Request
```bash
curl -X POST http://localhost:4000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

## 📂 Project Structure

```
backend/
├── cmd/
│   └── main.go              # Application entry point
├── docker/
│   └── Dockerfile           # Docker configuration
├── docs/                    # Swagger documentation (auto-generated)
├── envs/
│   └── .env                 # Environment variables
├── internal/
│   ├── config/              # Configuration and database setup
│   ├── constants/           # Application constants
│   ├── handlers/            # HTTP handlers
│   │   ├── auth/
│   │   ├── products/
│   │   └── users/
│   ├── middleware/          # Middleware (auth, etc.)
│   ├── model/               # Data models
│   ├── repository/          # Data access layer
│   │   ├── product/
│   │   └── user/
│   ├── routes/              # Route definitions
│   └── services/            # Business logic
│       ├── auth/
│       ├── cleanup/
│       ├── product/
│       └── user/
├── pkg/
│   ├── jwt/                 # JWT utilities
│   └── utils/               # Utility functions (S3, MinIO)
└── go.mod                   # Go modules
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

## 📦 Key Dependencies

- **Fiber v2** - Fast HTTP framework
- **GORM** - ORM library for PostgreSQL
- **JWT** - JSON Web Token authentication
- **AWS SDK v2** - S3/MinIO client
- **Redis** - Caching layer
- **RabbitMQ** - Message queue
- **Swagger** - API documentation
- **BCrypt** - Password hashing

## 🔧 Configuration

### Database Connection Pooling
The application uses connection pooling for optimal database performance:
- Max Open Connections: 25 (configurable)
- Max Idle Connections: 5 (configurable)
- Connection Max Lifetime: 5 minutes (configurable)

### Redis Caching
- Cache TTL: 5 minutes
- Pool Size: 10 connections
- Automatic cache invalidation on data changes

### File Storage
- Temporary uploads: `temp/` folder
- Product images: `products/{product-id}/` folder
- Automatic cleanup of temp files older than 24 hours
- Presigned URLs with 24-hour expiration

## 🐛 Error Handling

The API uses standard HTTP status codes and returns consistent error responses:

```json
{
  "status": "error",
  "code": 400,
  "message": "Invalid request"
}
```

## 🔒 Security Features

- Password hashing with BCrypt
- JWT token-based authentication
- CORS protection
- Rate limiting ready infrastructure
- Environment-based configuration
- Secure file upload validation

## 📊 Performance Optimization

- Redis caching for frequently accessed data
- Database connection pooling
- Gzip compression support
- Efficient database queries with GORM
- Lazy loading of relationships
- Presigned URLs for direct S3 access

## 🚀 Deployment

### Environment Variables for Production
Make sure to set strong values for:
- `JWT_SECRET` - Use a strong, random secret key
- `DB_PASSWORD` - Use a strong database password
- `AWS_ACCESS_KEY_ID` & `AWS_SECRET_ACCESS_KEY` - Use proper IAM credentials
- `CORS_ORIGINS` - Set specific allowed origins

### Recommended Production Settings
```env
APP_PORT=4000
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=10
REDIS_POOL_SIZE=20
CORS_ORIGINS=https://yourdomain.com
```

## 📝 License

This project is licensed under the Apache 2.0 License.

## 👤 Author

**Gani Ramadhan**
- Email: gani@example.com

## 🤝 Contributing

Contributions, issues, and feature requests are welcome!

## 📞 Support

If you have any questions or need help, please open an issue in the repository.
