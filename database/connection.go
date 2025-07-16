package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	_ "github.com/lib/pq"
)

var (
	connString = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", os.Getenv("USER_BD"), os.Getenv("PASS_BD"), os.Getenv("HOST_BD"), os.Getenv("PORT_BD"), os.Getenv("DBNAME"))
	conex      *sql.DB
	ctx        context.Context
)

var DBPool *sql.DB

func InitDB() error {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("USER_BD"),
		os.Getenv("PASS_BD"),
		os.Getenv("HOST_BD"),
		os.Getenv("PORT_BD"),
		os.Getenv("DBNAME"),
	)

	log.Println(connString)

	var err error
	DBPool, err = sql.Open("postgres", connString)
	if err != nil {

		log.Println("error haciendo ping a la base de datos: %w", err)
		cmd := exec.Command("sudo", "/bin/systemctl", "restart", "postgresql.service")
		_, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("Error:", err)
		}

		err = conex.Ping()
		if err != nil {

			return fmt.Errorf("error abriendo conexión: %w", err)
		}
	}

	DBPool.SetMaxOpenConns(100) // O incluso más, si tu base lo permite
	DBPool.SetMaxIdleConns(20)

	DBPool.SetConnMaxLifetime(30 * time.Minute) // Tiempo máximo de vida por conexión

	// Prueba de conexión
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := DBPool.PingContext(ctx); err != nil {

		log.Println("error haciendo ping a la base de datos: %w", err)
		cmd := exec.Command("sudo", "/bin/systemctl", "restart", "postgresql.service")
		_, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("Error:", err)
		}

		err = conex.Ping()
		if err != nil {
			return fmt.Errorf("error haciendo ping a la base de datos: %w", err)
		}
	}

	log.Println("✅ Base de datos conectada correctamente")
	return nil
}

// Connect : function to connect the database of califications but no return the conection

func GetDB() *sql.DB {
	if DBPool == nil {
		log.Println("🚨 DBPool es nil")
		return nil
	}

	// Verifica si la conexión sigue activa
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := DBPool.PingContext(ctx); err != nil {
		log.Printf("🚨 DBPool está presente, pero perdió conexión: %v\n", err)
		return nil
	}

	// 🔎 Mostrar estadísticas del pool
	/*stats := DBPool.Stats()

	log.Printf("📊 Pool stats → Open: %d | InUse: %d | Idle: %d | WaitCount: %d | MaxOpen: %d",
		stats.OpenConnections, stats.InUse, stats.Idle, stats.WaitCount, stats.MaxOpenConnections,
	)

	log.Println("✅ Conexión al pool activa")*/
	return DBPool
}

func Connect_old() (conn *sql.Conn, err error) {

	if conex == nil {

		log.Println("pase aqui")

		connString = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", os.Getenv("USER_BD"), os.Getenv("PASS_BD"), os.Getenv("HOST_BD"), os.Getenv("PORT_BD"), os.Getenv("DBNAME"))
		conex, err = sql.Open("postgres", connString)
		if err != nil {
			return
		}

		conex.SetConnMaxLifetime(time.Second * 30)
		conex.SetMaxIdleConns(0)
		conex.SetMaxOpenConns(200)
	}

	err = conex.Ping()
	if err != nil {
		log.Println("error al conectar con base de datos ")
		cmd := exec.Command("sudo", "/bin/systemctl", "restart", "postgresql.service")
		_, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("Error:", err)
		}

		err = conex.Ping()
	}

	ctx = context.TODO()
	conn, err = conex.Conn(ctx)
	if err != nil {
		log.Println("el error  callo aqui dos")
		return
	}

	return
}

// Query : function to make the query in the database
func Query(conn *sql.Conn, query string, data ...interface{}) (*sql.Rows, error) {
	return conn.QueryContext(ctx, query, data...)
}

// Exec : function to updates and deletes
func Exec(conn *sql.Conn, query string, data ...interface{}) (result sql.Result, err error) {
	return conn.ExecContext(ctx, query, data...)
}

// Close : function to close the connection with the database
func Close(conn *sql.Conn) error {
	return conn.Close()
}

func CloseDB() {
	if DBPool != nil {
		DBPool.Close()
	}
}
