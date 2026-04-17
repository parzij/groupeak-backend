package config

import (
	"log"
	"os"
)

type Config struct {
	DBDsn       string
	JWTToken    string
	HTTPPort    string
	AppDomain   string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
}

func Load() Config {
	httpPort := os.Getenv("APP_PORT")

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbSSL := os.Getenv("DB_SSLMODE")

	if dbUser == "" || dbName == "" {
		log.Fatal("DB_USER or DB_NAME must be set")
	}

	dsn := "postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName + "?sslmode=" + dbSSL

	jwtToken := os.Getenv("JWT_TOKEN")
	if jwtToken == "" {
		log.Fatal("JWT_TOKEN must be set")
	}

	appDomain := os.Getenv("APP_DOMAIN")

	s3Endpoint := os.Getenv("S3_ENDPOINT")
	s3Access := os.Getenv("S3_ACCESS_KEY")
	s3Secret := os.Getenv("S3_SECRET_KEY")
	s3Bucket := os.Getenv("S3_BUCKET_NAME")

	if s3Endpoint == "" || s3Access == "" || s3Secret == "" || s3Bucket == "" {
		log.Fatal("S3 environment variables must be set")
	}

	return Config{
		HTTPPort:    httpPort,
		DBDsn:       dsn,
		JWTToken:    jwtToken,
		AppDomain:   appDomain,
		S3Endpoint:  s3Endpoint,
		S3AccessKey: s3Access,
		S3SecretKey: s3Secret,
		S3Bucket:    s3Bucket,
	}
}
