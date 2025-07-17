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

	// Obtener nombre de la carpeta raíz
	rootName := obtenerNombreCarpeta(idFolder)
	safeName := strings.ReplaceAll(rootName, " ", "_")

	// Obtener estructura de carpetas
	folderList, err := obtenerCarpetas(idFolder, rootName)
	if err != nil {
		panic(err)
	}

	// Crear carpeta temporal con ID aleatorio
	rand.Seed(time.Now().UnixNano())
	randomID := rand.Intn(1000000)
	subDir := fmt.Sprintf("%s_%d", safeName, randomID)

	baseTemp := filepath.Join("/usr/bin/fd_cloud/temp", subDir)
	workingDir := filepath.Join(baseTemp, safeName)

	// Crear estructura base
	if err := exec.Command("sudo", "mkdir", "-p", workingDir).Run(); err != nil {
		panic(fmt.Sprintf("❌ No se pudo crear carpeta temporal: %v", err))
	}

	// Ruta final del ZIP
	finalZipPath := fmt.Sprintf("/usr/bin/fd_cloud/temp/%s.zip", safeName)

	// Obtener documentos
	documentList, err := obtenerDocumentos(idFolder)
	if err != nil {
		panic(err)
	}

	// Crear carpetas internas
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

	// Comprimir la carpeta raíz (no la carpeta temporal)
	cmd := exec.Command("sudo", "zip", "-r", finalZipPath, safeName)
	cmd.Dir = baseTemp
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Error creando ZIP: %s\n", out)
	} else {
		fmt.Println("✅ ZIP creado en:", finalZipPath)
	}
}

func obtenerNombreCarpeta(id int) string {
	conn := database.GetDB()
	var name string
	err := conn.QueryRow(`SELECT name FROM public.folder WHERE id = $1`, id).Scan(&name)
	if err != nil {
		panic(fmt.Sprintf("❌ No se pudo obtener el nombre de la carpeta con ID %d: %v", id, err))
	}
	return name
}

func obtenerCarpetas(idFolder int, rootName string) ([]map[string]string, error) {
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

	// Construcción de paths
	var buildPaths func(id string, currentPath string)
	results := []map[string]string{}

	buildPaths = func(id string, currentPath string) {
		f := folders[id]
		newPath := filepath.Join(currentPath, f.Name)

		// Remover el nombre raíz si se duplica
		relPath := strings.TrimPrefix(newPath, rootName)
		relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

		results = append(results, map[string]string{
			"id":      f.ID,
			"name":    f.Name,
			"path_is": relPath,
		})

		for _, childID := range children[f.ID] {
			buildPaths(childID, newPath)
		}
	}

	buildPaths(fmt.Sprintf("%d", idFolder), "")
	return results, nil
}

func obtenerDocumentos(idFolder int) ([]map[string]string, error) {
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
		SELECT id FROM folder_tree;
	`

	rows, err := conn.Query(query, idFolder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folderIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		folderIDs = append(folderIDs, id)
	}

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

	docRows, err := conn.Query(docQuery, pq.Array(folderIDs))
	if err != nil {
		return nil, err
	}
	defer docRows.Close()

	var result []map[string]string
	for docRows.Next() {
		var (
			id, agent, folderID, nameReal, nameSave string
		)
		if err := docRows.Scan(&id, &agent, &folderID, &nameReal, &nameSave); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{
			"id":        id,
			"name_real": nameReal,
			"name":      nameSave,
			"folder":    folderID,
			"path_is":   "", // se completa en main con la estructura de carpetas
		})
	}

	// Mapear paths a documentos
	folderPaths := make(map[string]string)
	for _, f := range obtenerPaths(folderIDs, idFolder) {
		folderPaths[f["id"]] = f["path_is"]
	}
	for i, doc := range result {
		result[i]["path_is"] = folderPaths[doc["folder"]]
	}

	return result, nil
}

func obtenerPaths(ids []string, idRoot int) []map[string]string {
	rootName := obtenerNombreCarpeta(idRoot)
	results, _ := obtenerCarpetas(idRoot, rootName)
	return results
}
