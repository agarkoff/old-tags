package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Укажите адрес Git-репозитория как аргумент")
	}
	repoURL := os.Args[1]

	// Создаем временную директорию
	tmpDir, err := os.MkdirTemp("", "git-tags-*")
	if err != nil {
		log.Fatalf("Не удалось создать временную директорию: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Клонируем репозиторий
	fmt.Println("Клонируем репозиторий...")
	cmd := exec.Command("git", "clone", "--quiet", "--mirror", repoURL, tmpDir)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Ошибка при клонировании репозитория: %v", err)
	}

	// Получаем список тегов с датами
	fmt.Println("Получаем список тегов и дат...")
	cmd = exec.Command("git", "for-each-ref",
		"--sort=creatordate",
		"--format=%(refname:short)|%(creatordate:iso8601)",
		"refs/tags")
	cmd.Dir = tmpDir
	output, err := cmd.Output()
	if err != nil {
		log.Fatalf("Ошибка при получении тегов: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		fmt.Println("Теги не найдены.")
		fmt.Println("Общее количество тегов: 0")
		return
	}

	fmt.Printf("%-30s %-25s %-15s\n", "Тег", "Дата создания", "Тип тега")
	fmt.Println(strings.Repeat("-", 75))

	// Выводим с определением типа
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		tagName := parts[0]
		created := parts[1]

		// Определяем тип тега
		typ := getTagType(tmpDir, tagName)

		fmt.Printf("%-30s %-25s %-15s\n", tagName, created, typ)
	}

	fmt.Printf("\nОбщее количество тегов: %d\n", len(lines))
}

func getTagType(repoPath, tag string) string {
	cmd := exec.Command("git", "cat-file", "-t", tag)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "неизвестно"
	}
	t := strings.TrimSpace(string(out))
	if t == "tag" {
		return "аннотированный"
	}
	if t == "commit" {
		return "легковесный"
	}
	return t
}
