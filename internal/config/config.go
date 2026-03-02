package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	DB          *gorm.DB
	RedisClient *redis.Client
	RabbitMQ    *amqp.Connection
	S3Client    *s3.Client
	S3Bucket    string
)

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func LoadEnv() {
	if root := findProjectRoot(); root != "" {
		envPath := filepath.Join(root, "envs", ".env")
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("Loaded environment from %s", envPath)
			return
		}
	}

	for _, p := range []string{"envs/.env", "../envs/.env"} {
		if err := godotenv.Load(p); err == nil {
			return
		}
	}

	log.Println("Warning: .env file not found, using system environment variables")
}

func GetEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func ConnectDatabase() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		GetEnv("DB_HOST", "localhost"),
		GetEnv("DB_PORT", "5432"),
		GetEnv("DB_USER", "postgres"),
		GetEnv("DB_PASSWORD", "postgres"),
		GetEnv("DB_NAME", "ganipedia"),
		GetEnv("DB_SSLMODE", "disable"),
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   gormlogger.Default.LogMode(gormlogger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
		PrepareStmt:                              true,
	})
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to get database instance: ", err)
	}

	maxOpenConns, _ := strconv.Atoi(GetEnv("DB_MAX_OPEN_CONNS", "25"))
	maxIdleConns, _ := strconv.Atoi(GetEnv("DB_MAX_IDLE_CONNS", "5"))
	maxLifetime, _ := strconv.Atoi(GetEnv("DB_CONN_MAX_LIFETIME", "5"))

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Fatal("Failed to ping database: ", err)
	}

	log.Println("Database connected successfully with connection pooling")
}

func ConnectRedis() error {
	redisHost := GetEnv("REDIS_HOST", "localhost")
	redisPort := GetEnv("REDIS_PORT", "6379")
	redisPassword := GetEnv("REDIS_PASSWORD", "")
	redisDB, _ := strconv.Atoi(GetEnv("REDIS_DB", "0"))
	poolSize, _ := strconv.Atoi(GetEnv("REDIS_POOL_SIZE", "10"))

	RedisClient = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password:     redisPassword,
		DB:           redisDB,
		PoolSize:     poolSize,
		MinIdleConns: 3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		RedisClient = nil
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("Redis connected successfully")
	return nil
}

func ConnectRabbitMQ() error {
	rabbitUser := GetEnv("RABBITMQ_USER", "guest")
	rabbitPass := GetEnv("RABBITMQ_PASSWORD", "guest")
	rabbitHost := GetEnv("RABBITMQ_HOST", "localhost")
	rabbitPort := GetEnv("RABBITMQ_PORT", "5672")

	rabbitURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", rabbitUser, rabbitPass, rabbitHost, rabbitPort)

	var err error
	RabbitMQ, err = amqp.Dial(rabbitURL)
	if err != nil {
		RabbitMQ = nil
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	log.Println("RabbitMQ connected successfully")
	return nil
}

func ConnectS3() {
	accessKey := GetEnv("AWS_ACCESS_KEY_ID", "")
	secretKey := GetEnv("AWS_SECRET_ACCESS_KEY", "")
	region := GetEnv("AWS_DEFAULT_REGION", "us-east-1")
	endpoint := GetEnv("AWS_ENDPOINT", "")
	usePathStyle := GetEnv("AWS_USE_PATH_STYLE_ENDPOINT", "true") == "true"

	S3Bucket = GetEnv("AWS_BUCKET", "ganipedia")

	if accessKey == "" || secretKey == "" {
		log.Fatal("AWS credentials are required")
	}

	cfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}

	if endpoint != "" {
		cfg.BaseEndpoint = aws.String(endpoint)
	}

	S3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = usePathStyle
	})

	ctx := context.Background()
	_, err := S3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(S3Bucket),
	})
	if err != nil {
		log.Printf("Bucket '%s' not found, attempting to create...", S3Bucket)
		_, createErr := S3Client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(S3Bucket),
		})
		if createErr != nil {
			log.Printf("Warning: Could not create bucket '%s': %v", S3Bucket, createErr)
			log.Println("Please create the bucket manually via MinIO console or mc client")
		} else {
			log.Printf("Bucket '%s' created successfully", S3Bucket)
		}
	}

	log.Println("S3/MinIO connected successfully")
}

func IsRedisConnected() bool {
	if RedisClient == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return RedisClient.Ping(ctx).Err() == nil
}

func IsRabbitMQConnected() bool {
	return RabbitMQ != nil && !RabbitMQ.IsClosed()
}

func CloseConnections() {
	if RedisClient != nil {
		if err := RedisClient.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
		}
	}

	if RabbitMQ != nil {
		if err := RabbitMQ.Close(); err != nil {
			log.Printf("Error closing RabbitMQ connection: %v", err)
		}
	}

	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				log.Printf("Error closing database connection: %v", err)
			}
		}
	}
}
