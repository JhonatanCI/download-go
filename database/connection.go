package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var (
	DBPool *sql.DB
	ctx    context.Context
)

// InitDB conecta a la base de datos usando variables del entorno
func InitDB() error {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	log.Println("DSN generado:", dsn)

	var err error
	DBPool, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("❌ error abriendo conexión: %w", err)
	}

	// Configuración del pool
	DBPool.SetMaxOpenConns(100)
	DBPool.SetMaxIdleConns(20)
	DBPool.SetConnMaxLifetime(30 * time.Minute)

	// Probar conexión
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := DBPool.PingContext(ctx); err != nil {
		return fmt.Errorf("❌ error haciendo ping a la base de datos: %w", err)
	}

	log.Println("✅ Base de datos conectada correctamente")
	return nil
}

// GetDB retorna la conexión activa
func GetDB() *sql.DB {
	if DBPool == nil {
		log.Println("🚨 DBPool es nil")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := DBPool.PingContext(ctx); err != nil {
		log.Printf("🚨 DBPool está presente, pero perdió conexión: %v", err)
		return nil
	}

	return DBPool
}

// CloseDB cierra la conexión
func CloseDB() {
	if DBPool != nil {
		DBPool.Close()
	}
}
