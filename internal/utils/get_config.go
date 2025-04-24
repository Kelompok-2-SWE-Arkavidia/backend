package utils

import (
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

type Config struct {
	// Database configuration
	DBUser     string `yaml:"DB_USER"`
	DBName     string `yaml:"DB_NAME"`
	DBPassword string `yaml:"DB_PASSWORD"`
	DBPort     string `yaml:"DB_PORT"`
	DBHost     string `yaml:"DB_HOST"`

	// JWT and AES Keys
	JWTSecret string `yaml:"JWT_SECRET"`
	AESKey    string `yaml:"AES_KEY"`

	// Mailing configuration
	AppURL           string `yaml:"APP_URL"`
	SMTPHost         string `yaml:"SMTP_HOST"`
	SMTPPort         string `yaml:"SMTP_PORT"`
	SMTPSenderName   string `yaml:"SMTP_SENDER_NAME"`
	SMTPAuthEmail    string `yaml:"SMTP_AUTH_EMAIL"`
	SMTPAuthPassword string `yaml:"SMTP_AUTH_PASSWORD"`

	// Midtrans configuration
	ClientKey string `yaml:"CLIENT_KEY"`
	ServerKey string `yaml:"SERVER_KEY"`
	IsProd    bool   `yaml:"IsProd"`

	// AWS S3 configuration
	AWSS3Bucket  string `yaml:"AWS_S3_BUCKET"`
	AWSS3Region  string `yaml:"AWS_S3_REGION"`
	AWSAccessKey string `yaml:"AWS_ACCESS_KEY"`
	AWSSecretKey string `yaml:"AWS_SECRET_KEY"`

	// Gemini API configuration
	GeminiAPIKey string `yaml:"GEMINI_API_KEY"`
	GeminiModel  string `yaml:"GEMINI_MODEL"`

	// AI Model Service
	AIModelURL string `yaml:"AI_MODEL_URL"`
}

var config Config

func LoadConfig() {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Printf("Error reading YAML file: %s\n", err)
		return
	}

	err = yaml.Unmarshal(file, &config)
	if err != nil {
		log.Printf("Error parsing YAML file: %s\n", err)
		return
	}

	// Set environment variables for keys that should be accessible via os.Getenv
	os.Setenv("JWT_SECRET", config.JWTSecret)
	os.Setenv("AES_KEY", config.AESKey)
	os.Setenv("SERVER_KEY", config.ServerKey)
	os.Setenv("CLIENT_KEY", config.ClientKey)
	os.Setenv("IS_PROD", getBoolString(config.IsProd))
	os.Setenv("AWS_S3_BUCKET", config.AWSS3Bucket)
	os.Setenv("AWS_S3_REGION", config.AWSS3Region)
	os.Setenv("AWS_ACCESS_KEY", config.AWSAccessKey)
	os.Setenv("AWS_SECRET_KEY", config.AWSSecretKey)
	os.Setenv("GEMINI_API_KEY", config.GeminiAPIKey)
	os.Setenv("AI_MODEL_URL", config.AIModelURL)
}

func getBoolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func GetConfig(key string) string {
	switch key {
	case "DB_USER":
		return config.DBUser
	case "DB_NAME":
		return config.DBName
	case "DB_PASSWORD":
		return config.DBPassword
	case "DB_PORT":
		return config.DBPort
	case "DB_HOST":
		return config.DBHost
	case "JWT_SECRET":
		return config.JWTSecret
	case "AES_KEY":
		return config.AESKey
	case "APP_URL":
		return config.AppURL
	case "SMTP_HOST":
		return config.SMTPHost
	case "SMTP_PORT":
		return config.SMTPPort
	case "SMTP_SENDER_NAME":
		return config.SMTPSenderName
	case "SMTP_AUTH_EMAIL":
		return config.SMTPAuthEmail
	case "SMTP_AUTH_PASSWORD":
		return config.SMTPAuthPassword
	case "CLIENT_KEY":
		return config.ClientKey
	case "SERVER_KEY":
		return config.ServerKey
	case "IsProd":
		if config.IsProd {
			return "true"
		}
		return "false"
	case "AWS_S3_BUCKET":
		return config.AWSS3Bucket
	case "AWS_S3_REGION":
		return config.AWSS3Region
	case "AWS_ACCESS_KEY":
		return config.AWSAccessKey
	case "AWS_SECRET_KEY":
		return config.AWSSecretKey
	case "GEMINI_API_KEY":
		return config.GeminiAPIKey
	case "GEMINI_MODEL":
		return config.GeminiModel
	case "AI_MODEL_URL":
		return config.AIModelURL
	default:
		return ""
	}
}
