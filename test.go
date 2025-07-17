package main

import (
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"tuproyecto/database"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
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

	// Obtener estructura de carpetas para saber el nombre raíz
	folderList, err := obtenerCarpetas(idFolder)
	if err != nil {
		panic(err)
	}

	var rootName string
	for _, f := range folderList {
		if f["id"] == fmt.Sprintf("%d", idFolder) {
			rootName = f["name"]
			break
		}
	}
	if rootName == "" {
		panic(fmt.Sprintf("❌ No se encontró el nombre de la carpeta con ID %d", idFolder))
	}

	// Obtener nombre de la carpeta raíz
	safeName := strings.ReplaceAll(rootName, " ", "_")

	// Carpeta temporal base con ID aleatorio
	rand.Seed(time.Now().UnixNano())
	randomID := rand.Intn(1000000)
	subDir := fmt.Sprintf("%s_%d", safeName, randomID)

	baseTemp := filepath.Join("/tmp", subDir)
	workingDir := filepath.Join(baseTemp, safeName) // Aquí irá nivel_2

	// Crear estructura base
	if err := exec.Command("sudo", "mkdir", "-p", workingDir).Run(); err != nil {
		panic(fmt.Sprintf("❌ No se pudo crear carpeta temporal de trabajo: %v", err))
	}

	// Ruta final del ZIP
	finalZipPath := fmt.Sprintf("/usr/bin/fd_cloud/temp/%s.zip", safeName)

	// Crear carpeta temporal
	if err := exec.Command("sudo", "mkdir", "-p", workingDir).Run(); err != nil {
		panic(fmt.Sprintf("❌ No se pudo crear carpeta temporal de trabajo: %v", err))
	}

	// Obtener documentos
	documentList, err := obtenerDocumentos(idFolder)
	if err != nil {
		panic(err)
	}

	// Crear carpetas dentro del workingDir
	for _, f := range folderList {
		fullPath := filepath.Join(workingDir, f["path_is"])
		if err := exec.Command("sudo", "mkdir", "-p", fullPath).Run(); err != nil {
			fmt.Println("❌ Error creando carpeta:", fullPath, err)
		}
	}

	// Copiar documentos
	for _, doc := range documentList {
		origin := filepath.Join("/usr/bin/fd_cloud/temp/", doc["name"])
		dest := filepath.Join(workingDir, doc["path_is"], doc["name_real"])

		cmd := exec.Command("sudo", "cp", origin, dest)
		_, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("⚠️  Archivo no encontrado, creando archivo vacío: %s → %s\n", origin, dest)

			tempEmpty := filepath.Join("/tmp", "empty_temp_file")
			_ = exec.Command("sudo", "touch", tempEmpty).Run()

			cpEmpty := exec.Command("sudo", "cp", tempEmpty, dest)
			_, err2 := cpEmpty.CombinedOutput()
			if err2 != nil {
				fmt.Printf("❌ Error copiando archivo vacío a destino: %s\n", dest)
			}
		}
	}

	// Comprimir subcarpeta
	cmd := exec.Command("sudo", "zip", "-r", finalZipPath, safeName)
	cmd.Dir = baseTemp
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
		WITH RECURSIVE folder_tree AS (
			SELECT id, father, name
			FROM public.folder
			WHERE id = $1 AND delete = false

			UNION ALL

			SELECT f.id, f.father, f.name
			FROM public.folder f
			INNER JOIN folder_tree ft ON f.father = ft.id
			WHERE f.delete = false
		)
		SELECT id, father, name FROM folder_tree;
	`

	rows, err := conn.Query(query, idFolder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type folder struct {
		ID     string
		Father string
		Name   string
	}

	// Map de ID a carpeta
	folders := map[string]folder{}
	// Map de ID a hijos
	children := map[string][]string{}

	for rows.Next() {
		var f folder
		if err := rows.Scan(&f.ID, &f.Father, &f.Name); err != nil {
			return nil, err
		}
		folders[f.ID] = f
		children[f.Father] = append(children[f.Father], f.ID)
	}

	// Generar path relativo
	var buildPaths func(id string, currentPath string)
	results := []map[string]string{}

	buildPaths = func(id string, currentPath string) {
		f := folders[id]
		newPath := filepath.Join(currentPath, f.Name)

		results = append(results, map[string]string{
			"id":      f.ID,
			"name":    f.Name,
			"path_is": newPath,
		})

		for _, childID := range children[f.ID] {
			buildPaths(childID, newPath)
		}
	}

	// Comenzar desde el idFolder como raíz
	buildPaths(fmt.Sprintf("%d", idFolder), "")

	return results, nil
}

func obtenerDocumentos(idFolder int) ([]map[string]string, error) {
	conn := database.GetDB()

	// 1. Traemos todos los folders necesarios (padres e hijos)
	folderQuery := `
		WITH RECURSIVE folder_tree AS (
			SELECT id, father, name
			FROM public.folder
			WHERE id = $1 AND delete = false

			UNION ALL

			SELECT f.id, f.father, f.name
			FROM public.folder f
			INNER JOIN folder_tree ft ON f.father = ft.id
			WHERE f.delete = false
		)
		SELECT id, father, name FROM folder_tree;
	`

	rows, err := conn.Query(folderQuery, idFolder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type folder struct {
		ID     string
		Father string
		Name   string
	}

	folders := map[string]folder{}
	children := map[string][]string{}

	for rows.Next() {
		var f folder
		if err := rows.Scan(&f.ID, &f.Father, &f.Name); err != nil {
			return nil, err
		}
		folders[f.ID] = f
		children[f.Father] = append(children[f.Father], f.ID)
	}

	// Generar path relativo de folders
	folderPaths := map[string]string{}

	var buildPaths func(id string, currentPath string)
	buildPaths = func(id string, currentPath string) {
		f := folders[id]
		newPath := filepath.Join(currentPath, f.Name)
		folderPaths[id] = newPath

		for _, childID := range children[f.ID] {
			buildPaths(childID, newPath)
		}
	}
	buildPaths(fmt.Sprintf("%d", idFolder), "")

	// 2. Traemos los documentos de esos folders
	docQuery := `
		SELECT 
			d.id,
			d.agent,
			d.folder,
			doc_data->>'name' AS name_real,
			d.name AS name
		FROM public.document d
		WHERE d.delete = false AND d.trash = false AND d.folder = ANY($1)
	`

	// Construir lista de folder IDs
	var folderIDs []interface{}
	for folderID := range folderPaths {
		folderIDs = append(folderIDs, folderID)
	}

	// Convierte folderIDs a []string para la consulta
	folderIDsStr := make([]string, len(folderIDs))
	for i, id := range folderIDs {
		folderIDsStr[i] = fmt.Sprintf("%v", id)
	}

	// Ejecutar consulta
	docRows, err := conn.Query(docQuery, pq.Array(folderIDsStr))
	if err != nil {
		return nil, err
	}
	defer docRows.Close()

	type document struct {
		ID        string
		Agent     string
		FolderID  string
		NameReal  string
		NameSave  string
	}

	var result []map[string]string

	for docRows.Next() {
		var d document
		if err := docRows.Scan(&d.ID, &d.Agent, &d.FolderID, &d.NameReal, &d.NameSave); err != nil {
			return nil, err
		}

		path := folderPaths[d.FolderID]

		result = append(result, map[string]string{
			"id":        d.ID,
			"name_real": d.NameReal,
			"name":      d.NameSave,
			"folder":    d.FolderID,
			"path_is":   path,
		})
	}

	return result, nil
}
