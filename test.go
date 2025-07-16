package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"tuproyecto/database"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		panic("No se pudo cargar el archivo .env: " + err.Error())
	}
	if err := database.InitDB(); err != nil {
		panic("❌ Error conectando a la base de datos: " + err.Error())
	}
	defer database.CloseDB()

	idFolder := 3

	// Carpeta temporal
	tempDir := "/tmp/expediente_temp/"

	// Carpeta final donde guardar el ZIP
	finalZipPath := "/usr/bin/fd_cloud/temp/expediente.zip"

	// Crear carpeta temporal
	if err := exec.Command("sudo", "mkdir", "-p", tempDir).Run(); err != nil {
		panic(fmt.Sprintf("❌ No se pudo crear carpeta temporal: %v", err))
	}

	folderList, err := obtenerCarpetas(idFolder)
	if err != nil {
		panic(err)
	}

	documentList, err := obtenerDocumentos(idFolder)
	if err != nil {
		panic(err)
	}

	// Crear carpetas dentro de /tmp
	for _, f := range folderList {
		fullPath := filepath.Join(tempDir, f["path_is"])
		if err := exec.Command("sudo", "mkdir", "-p", fullPath).Run(); err != nil {
			fmt.Println("❌ Error creando carpeta:", fullPath, err)
		}
	}

	// Mover archivos desde /usr/bin/fd_cloud/public a la carpeta temporal
	for _, doc := range documentList {
		origin := filepath.Join("/usr/bin/fd_cloud/public/", doc["name"])
		dest := filepath.Join(tempDir, doc["path_is"], doc["name_real"])

		cmd := exec.Command("sudo", "cp", origin, dest)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ Error copiando archivo %s → %s → %s\n", origin, dest, out)
		}
	}

	// Crear el ZIP final en la carpeta pública
	cmd := exec.Command("sudo", "zip", "-r", finalZipPath, ".")
	cmd.Dir = tempDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Error creando ZIP: %s\n", out)
	} else {
		fmt.Println("✅ ZIP creado en:", finalZipPath)
	}
}


func obtenerCarpetas(idFolder int) ([]map[string]string, error) {
	conn := database.GetDB()
	query := `
		with y as (
			with y as (
				SELECT JSONB_ARRAY_ELEMENTS(f.tree)::INT "id"
				FROM public.folder f WHERE father = $1 and f.delete = false
			),
			ids as (
				SELECT id FROM public.folder f WHERE father = $1 or id = $1 and f.delete = false
			)
			SELECT id from y
			UNION
			SELECT id from ids
		)
		SELECT ARRAY_TO_JSON(ARRAY_AGG(ROW_TO_JSON(q1))) FROM (
			SELECT 
				f.id::text,
				f.path::text,
				f.name,
				(CASE WHEN f.path = '/' THEN f.path || f.name ELSE f.path || '/' || f.name END) "path_is"
			FROM public.folder f
			WHERE f.id in (select id from y) and f.delete = false
		) q1`

	var data sql.NullString
	err := conn.QueryRow(query, idFolder).Scan(&data)
	if err != nil {
		return nil, err
	}

	var temp []map[string]interface{}
	if err := json.Unmarshal([]byte(data.String), &temp); err != nil {
		return nil, err
	}

	var result []map[string]string
	for _, item := range temp {
		m := make(map[string]string)
		for k, v := range item {
			m[k] = fmt.Sprintf("%v", v)
		}
		result = append(result, m)
	}
	return result, nil
}

func obtenerDocumentos(idFolder int) ([]map[string]string, error) {
	conn := database.GetDB()
	query := `
		with y as (
			with y as (
				SELECT JSONB_ARRAY_ELEMENTS(f.tree)::INT "id"
				FROM public.folder f WHERE father = $1 and f.delete = false
			),
			ids as (
				SELECT id FROM public.folder f WHERE father = $1 or id = $1 and f.delete = false
			)
			SELECT id from y
			UNION
			SELECT id from ids
		)
		SELECT JSON_AGG(ROW_TO_JSON(q)) FROM (
			SELECT 
				d.id,
				d.agent,
				d.folder,
				doc_data->>'name' AS name_real,
				d.name AS name,
				(CASE WHEN f.path = '/' THEN f.path || f.name ELSE f.path || '/' || f.name END) AS path_is
			FROM public.document d
			INNER JOIN public.folder f ON f.id = d.folder
			WHERE d.delete = false AND d.trash = false AND f.id IN (SELECT id FROM y)
		) q`

	var data sql.NullString
	err := conn.QueryRow(query, idFolder).Scan(&data)
	if err != nil {
		return nil, err
	}

	var temp []map[string]interface{}
	if err := json.Unmarshal([]byte(data.String), &temp); err != nil {
		return nil, err
	}

	var result []map[string]string
	for _, item := range temp {
		m := make(map[string]string)
		for k, v := range item {
			m[k] = fmt.Sprintf("%v", v)
		}
		result = append(result, m)
	}
	return result, nil
}
