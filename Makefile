# Переменные конфигурации
CLUSTER_NAME ?= notion-clone
IMAGE_NAME   ?= my-registry/gateway
IMAGE_TAG    ?= local-dev
NAMESPACE    ?= default

# Цвета для вывода в терминал
CYAN   := \033[36m
GREEN  := \033[32m
RESET  := \033[0m

.PHONY: all generate test compose-up compose-down compose-logs cluster-up cluster-down build load deploy-k8s port-forward logs clean help

all: build load deploy-k8s ## Полный цикл для K8s: сборка, загрузка в Kind и деплой манифестов

generate: ## Перегенерировать whitelist HTTP/gRPC сервисов
	go generate ./internal/registry

test: generate ## Перегенерировать registry и запустить тесты
	go test ./...

help: ## Показать это справочное сообщение
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-15s$(RESET) %s\n", $$1, $$2}'

compose-up: ## Запустить гейтвей и mock-сервис локально в Docker Compose
	@echo "$(CYAN)Запуск окружения в Docker Compose...$(RESET)"
	docker compose up -d --build
	@echo "$(GREEN)Гейтвей доступен по адресу: http://localhost:8080$(RESET)"
	@echo "$(GREEN)Попробуйте сделать запрос: curl http://localhost:8080/api/v1/mock-service/test$(RESET)"

compose-down: ## Остановить окружение Docker Compose и очистить volumes
	@echo "$(CYAN)Остановка окружения Docker Compose...$(RESET)"
	docker compose down -v --remove-orphans

compose-logs: ## Посмотреть логи Docker Compose
	docker compose logs -f api-gateway

cluster-up: ## Создать локальный кластер Kind
	@echo "$(CYAN)Создание кластера Kind [$(CLUSTER_NAME)]...$(RESET)"
	kind create cluster --name $(CLUSTER_NAME) --config=deployments/kind-config.yaml || true

cluster-down: ## Удалить локальный кластер Kind
	@echo "$(CYAN)Удаление кластера Kind [$(CLUSTER_NAME)]...$(RESET)"
	kind delete cluster --name $(CLUSTER_NAME)

build: ## Скомпилировать Go-код в Docker-образ
	@echo "$(CYAN)Сборка Docker-образа $(IMAGE_NAME):$(IMAGE_TAG)...$(RESET)"
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

load: ## Загрузить локальный Docker-образ в кластер Kind
	@echo "$(CYAN)Загрузка образа в кластер Kind...$(RESET)"
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name $(CLUSTER_NAME)

deploy-k8s: ## Применить манифесты Deployment и Service в K8s
	@echo "$(CYAN)Деплой гейтвея в Kubernetes...$(RESET)"
	kubectl apply -f deployments/deployment.yaml -n $(NAMESPACE)
	kubectl apply -f deployments/service.yaml -n $(NAMESPACE)
	@echo "$(GREEN)Ожидание готовности подов (Rolling Update)...$(RESET)"
	kubectl rollout status deployment/api-gateway -n $(NAMESPACE) --timeout=90s

port-forward: ## Пробросить порт гейтвея на localhost:8080
	@echo "$(GREEN)Проброс портов запущен. Гейтвей доступен по адресу: http://localhost:8080$(RESET)"
	kubectl port-forward svc/api-gateway 8080:80 -n $(NAMESPACE)

logs: ## Посмотреть цветные логи гейтвея (требуется stern)
	@echo "$(CYAN)Подключение к логам api-gateway...$(RESET)"
	@if command -v stern >/dev/null 2>&1; then \
		stern api-gateway --namespace $(NAMESPACE) --color always; \
	else \
		echo "$(CYAN)Утилита stern не найдена. Используем стандартный kubectl logs...$(RESET)"; \
		kubectl logs -l app=api-gateway -n $(NAMESPACE) -f --tail=100; \
	fi

clean: ## Удалить деплоймент и сервис из кластера (с таймаутом на Graceful Shutdown)
	@echo "$(CYAN)Удаление ресурсов из K8s...$(RESET)"
	kubectl delete -f deployments/deployment.yaml -n $(NAMESPACE) --ignore-not-found --timeout=30s
	kubectl delete -f deployments/service.yaml -n $(NAMESPACE) --ignore-not-found --timeout=10s
