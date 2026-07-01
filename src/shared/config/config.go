package config

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Resend   ResendConfig
	Redis    RedisConfig
	Metrics  MetricsConfig
	Contact  ContactConfig
	SQS      SQSConfig
	EventBus EventBusConfig
}

type ServerConfig struct {
	Port int
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string `mapstructure:"ssl_mode"`
}

type ResendConfig struct {
	APIKey     string `mapstructure:"api_key"`
	FromEmail  string `mapstructure:"from_email"`
	MockSender bool   `mapstructure:"mock_sender"`
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type MetricsConfig struct {
	Enabled bool
	Port    int
}

type ContactConfig struct {
	Email string `mapstructure:"email"`
}

type SQSConfig struct {
	QueueURL string `mapstructure:"queue_url"`
	Region   string `mapstructure:"region"`
	Enabled  bool   `mapstructure:"enabled"`
}

// EventBusConfig apunta a la DB del EventBus PostgreSQL (compartida con los publishers).
// Enabled gatea el arranque del worker consumer: opt-in en prod recién cuando los secrets
// EVENTBUS_DB_* estén cableados, para no crashear/colgar el pod antes.
type EventBusConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	viper.AutomaticEnv()

	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("server.mode", "SERVER_MODE")

	viper.BindEnv("database.host", "DATABASE_HOST")
	viper.BindEnv("database.port", "DATABASE_PORT")
	viper.BindEnv("database.user", "DATABASE_USER")
	viper.BindEnv("database.password", "DATABASE_PASSWORD")
	viper.BindEnv("database.name", "DATABASE_NAME")
	viper.BindEnv("database.ssl_mode", "DATABASE_SSL_MODE")

	viper.BindEnv("resend.api_key", "RESEND_API_KEY")
	viper.BindEnv("resend.from_email", "RESEND_FROM_EMAIL")
	viper.BindEnv("resend.mock_sender", "RESEND_MOCK_SENDER")

	viper.BindEnv("contact.email", "CONTACT_EMAIL")

	viper.BindEnv("redis.host", "REDIS_HOST")
	viper.BindEnv("redis.port", "REDIS_PORT")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")

	viper.BindEnv("metrics.enabled", "METRICS_ENABLED")
	viper.BindEnv("metrics.port", "METRICS_PORT")

	viper.BindEnv("sqs.queue_url", "SQS_QUEUE_URL")
	viper.BindEnv("sqs.region", "SQS_REGION")
	viper.BindEnv("sqs.enabled", "SQS_ENABLED")

	viper.BindEnv("eventbus.enabled", "EVENTBUS_ENABLED")
	viper.BindEnv("eventbus.host", "EVENTBUS_DB_HOST")
	viper.BindEnv("eventbus.port", "EVENTBUS_DB_PORT")
	viper.BindEnv("eventbus.user", "EVENTBUS_DB_USER")
	viper.BindEnv("eventbus.password", "EVENTBUS_DB_PASSWORD")
	viper.BindEnv("eventbus.name", "EVENTBUS_DB_NAME")
	viper.BindEnv("eventbus.ssl_mode", "EVENTBUS_DB_SSL_MODE")

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("No config.yaml found, using only environment variables: %v", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
