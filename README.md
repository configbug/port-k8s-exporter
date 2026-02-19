<img align="right" width="100" height="74" src="https://user-images.githubusercontent.com/8277210/183290025-d7b24277-dfb4-4ce1-bece-7fe0ecd5efd4.svg" />

# Port K8s Exporter

[![Slack](https://img.shields.io/badge/Slack-4A154B?style=for-the-badge&logo=slack&logoColor=white)](https://join.slack.com/t/devex-community/shared_invite/zt-1bmf5621e-GGfuJdMPK2D8UN58qL4E_g)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go&logoColor=white)]()
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge)](LICENSE)

Port is the Developer Platform meant to supercharge your DevOps and Developers, and allow you to regain control of your environment. Este exportador sincroniza automáticamente recursos de Kubernetes con el catálogo de Port.

## 📋 Tabla de Contenidos

- [Descripción General](#descripción-general)
- [Arquitectura](#arquitectura)
- [Modos de Operación (Event Listeners)](#modos-de-operación-event-listeners)
- [Instalación](#instalación)
- [Configuración](#configuración)
- [Desarrollo Local](#desarrollo-local)
- [Observabilidad](#observabilidad)
- [Troubleshooting](#troubleshooting)

## Descripción General

Port K8s Exporter es un agente escrito en Go que se ejecuta dentro de tu cluster Kubernetes y sincroniza recursos K8s (Deployments, Services, Pods, ConfigMaps, Secrets, CRDs, etc.) hacia el catálogo de software de [Port](https://getport.io).

### Características Principales

- **Sincronización en tiempo real**: Detecta cambios en recursos K8s usando informers nativos
- **Transformación flexible**: Usa expresiones JQ para mapear recursos K8s a entidades de Port
- **Múltiples modos de operación**: POLLING, KAFKA, y WEBHOOK
- **Soporte multi-arquitectura**: Builds para `amd64` y `arm64`
- **Integración nativa**: Funciona con cualquier recurso K8s incluyendo CRDs
- **Alta disponibilidad**: Diseñado para ejecutarse como Deployment en K8s

## Arquitectura

```
┌─────────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CLUSTER                          │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    Port K8s Exporter                          │  │
│  │                                                               │  │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐   │  │
│  │  │   Config    │───▶│ Controllers │───▶│   Event Handler │   │  │
│  │  │   Loader    │    │  (per kind) │    │                 │   │  │
│  │  └─────────────┘    └──────┬──────┘    └────────┬────────┘   │  │
│  │                            │                     │            │  │
│  │                     ┌──────▼──────┐             │            │  │
│  │                     │  Informers  │             │            │  │
│  │                     │  (Watch K8s)│             │            │  │
│  │                     └──────┬──────┘             │            │  │
│  │                            │                     │            │  │
│  │                     ┌──────▼──────┐      ┌──────▼──────┐     │  │
│  │                     │ JQ Mapping  │      │   POLLING   │     │  │
│  │                     │ (Transform) │      │   KAFKA     │     │  │
│  │                     └──────┬──────┘      │   WEBHOOK   │     │  │
│  │                            │             └──────┬──────┘     │  │
│  │                            │                     │            │  │
│  └────────────────────────────┼─────────────────────┼────────────┘  │
│                               │                     │               │
└───────────────────────────────┼─────────────────────┼───────────────┘
                                │                     │
                                ▼                     ▼
                    ┌───────────────────────────────────────┐
                    │              PORT PLATFORM            │
                    │  ┌─────────────┐  ┌───────────────┐  │
                    │  │  REST API   │  │ Ingest Webhook│  │
                    │  │ api.getport │  │ingest.getport │  │
                    │  └─────────────┘  └───────────────┘  │
                    └───────────────────────────────────────┘
```

### Flujo de Datos

1. **Inicialización**: El exporter carga la configuración y crea controladores para cada tipo de recurso K8s configurado
2. **Watch**: Los informers de Kubernetes observan cambios en los recursos configurados
3. **Transformación**: Cuando ocurre un cambio, se aplican expresiones JQ para mapear el recurso K8s a una entidad de Port
4. **Sincronización**: Las entidades se envían a Port vía API REST o Webhook
5. **Resync**: Periódicamente se realiza una sincronización completa para garantizar consistencia

### Componentes Principales

| Componente | Ubicación | Descripción |
|------------|-----------|-------------|
| **Config** | `pkg/config/` | Carga y valida configuración desde flags, env vars y archivos |
| **K8s Controller** | `pkg/k8s/` | Gestiona informers y procesa eventos de K8s |
| **JQ Parser** | `pkg/jq/` | Ejecuta expresiones JQ para transformación de datos |
| **Port Client** | `pkg/port/cli/` | Cliente HTTP para la API de Port |
| **Webhook Client** | `pkg/port/webhook/` | Cliente HTTP para el endpoint de ingest |
| **Event Handlers** | `pkg/event_handler/` | Implementaciones de POLLING, KAFKA, WEBHOOK |
| **Handlers** | `pkg/handlers/` | Orquesta múltiples controladores |
| **Defaults** | `pkg/defaults/` | Configuración por defecto para blueprints, pages, scorecards |

## Modos de Operación (Event Listeners)

El exporter soporta **tres modos de operación** configurables mediante `eventListener.type`:

### 1. POLLING (Por defecto)

```yaml
eventListener:
  type: POLLING
```

**Características:**
- Consulta periódicamente la API de Port para detectar solicitudes de resync
- Requiere credenciales de Port (`PORT_CLIENT_ID`, `PORT_CLIENT_SECRET`)
- El mapeo JQ se ejecuta **localmente** en el exporter
- Ideal para: Entornos con conectividad bidireccional a Port

**Flujo:**
```
K8s Events → Informers → JQ Mapping (local) → Port REST API
                                  ↑
Port API ← Poll for resync requests (cada N segundos)
```

### 2. KAFKA

```yaml
eventListener:
  type: KAFKA
```

**Características:**
- Consume eventos del changelog de Port vía Kafka
- Requiere credenciales de Port para obtener credenciales de Kafka
- El mapeo JQ se ejecuta **localmente** en el exporter
- Ideal para: Alta frecuencia de eventos, baja latencia

**Flujo:**
```
K8s Events → Informers → JQ Mapping (local) → Port REST API
                                  ↑
Port Kafka ← Consume changelog events (tiempo real)
```

### 3. WEBHOOK (Nuevo)

```yaml
eventListener:
  type: WEBHOOK
```

**Características:**
- Envía recursos K8s crudos directamente a `ingest.getport.io`
- **NO requiere credenciales de Port** (`PORT_CLIENT_ID`, `PORT_CLIENT_SECRET`)
- El mapeo JQ se ejecuta **en Port** (no en el exporter)
- Envío en batches para optimizar rendimiento
- Ideal para: Entornos sin conectividad de retorno a Port, simplicidad de configuración

**Flujo:**
```
K8s Events → Informers → Batch → POST ingest.getport.io/integration/{id}/...
                           │
                           └─ Envía objetos K8s crudos (sin JQ local)
                                      │
                                      ▼
                              Port ejecuta JQ mapping
```

**Configuración específica para WEBHOOK:**

| Variable | Descripción | Default |
|----------|-------------|---------|
| `WEBHOOK_URL` | URL del endpoint de ingest | `https://ingest.getport.io` |
| `WEBHOOK_BATCH_SIZE` | Tamaño máximo del batch | `100` |
| `WEBHOOK_BATCH_TIMEOUT` | Timeout para flush del batch | `5s` |
| `CLUSTER_NAME` | Nombre identificador del cluster | (requerido) |
| `STATE_KEY` | Clave única para el estado de sincronización | (requerido) |

**Advanced Settings (Seguridad del Webhook):**

Estos parámetros permiten configurar la firma HMAC de los requests para verificar autenticidad:

| Variable | Descripción | Default |
|----------|-------------|---------|
| `WEBHOOK_SECRET` | Secreto para firma HMAC | (opcional) |
| `WEBHOOK_SIGNATURE_HEADER` | Nombre del header de firma | `X-Port-Signature` |
| `WEBHOOK_SIGNATURE_ALGORITHM` | Algoritmo: `sha256`, `sha1`, `sha512` | `sha256` |
| `WEBHOOK_SIGNATURE_PREFIX` | Prefijo para el valor de firma | (vacío) |
| `WEBHOOK_REQUEST_IDENTIFIER` | Path JQ para identificador de request | (opcional) |

> **Nota**: Estos valores deben coincidir con la configuración del webhook en Port (ver imagen de configuración avanzada en Port UI).

**Ejemplo de payload enviado a Port:**

```json
{
  "integration_id": "my-k8s-integration",
  "cluster_name": "production-cluster",
  "state_key": "prod-state",
  "entities": [
    {
      "action": "upsert",
      "kind": "apps/v1/deployments",
      "object": {
        "apiVersion": "apps/v1",
        "kind": "Deployment",
        "metadata": { "name": "my-app", "namespace": "default" },
        "spec": { ... }
      }
    }
  ]
}
```

### Comparativa de Modos

| Característica | POLLING | KAFKA | WEBHOOK |
|----------------|---------|-------|---------|
| Credenciales Port | ✅ Requeridas | ✅ Requeridas | ❌ No requeridas |
| JQ Mapping | Local | Local | En Port |
| Latencia resync | Media (polling interval) | Baja (tiempo real) | N/A (push only) |
| Complejidad | Baja | Media | Muy baja |
| Conectividad | Bidireccional | Bidireccional | Unidireccional (push) |
| Batch entities | No | No | Sí |

## Instalación

### Usando Helm (Recomendado)

```bash
# Agregar el repositorio de Helm
helm repo add port-labs https://port-labs.github.io/helm-charts
helm repo update

# Instalar con modo POLLING (default)
helm install port-k8s-exporter port-labs/port-k8s-exporter \
  --set secret.secrets.portClientId="YOUR_CLIENT_ID" \
  --set secret.secrets.portClientSecret="YOUR_CLIENT_SECRET" \
  --set configMap.config="$(cat your-config.yaml)"

# Instalar con modo KAFKA
helm install port-k8s-exporter port-labs/port-k8s-exporter \
  --set secret.secrets.portClientId="YOUR_CLIENT_ID" \
  --set secret.secrets.portClientSecret="YOUR_CLIENT_SECRET" \
  --set eventListener.type="KAFKA" \
  --set configMap.config="$(cat your-config.yaml)"

# Instalar con modo WEBHOOK (sin credenciales)
helm install port-k8s-exporter port-labs/port-k8s-exporter \
  --set eventListener.type="WEBHOOK" \
  --set stateKey="my-cluster-state" \
  --set clusterName="production" \
  --set configMap.config="$(cat your-config.yaml)"

# Instalar con modo WEBHOOK + Advanced Settings (seguridad)
helm install port-k8s-exporter port-labs/port-k8s-exporter \
  --set eventListener.type="WEBHOOK" \
  --set stateKey="my-cluster-state" \
  --set clusterName="production" \
  --set webhook.url="https://ingest.getport.io/your-webhook-id" \
  --set webhook.secret="your-hmac-secret" \
  --set webhook.signatureHeaderName="X-Port-Signature" \
  --set webhook.signatureAlgorithm="sha256" \
  --set webhook.signaturePrefix="sha256=" \
  --set webhook.batchSize=50 \
  --set webhook.batchTimeout=5 \
  --set configMap.config="$(cat your-config.yaml)"
```

### Chart de Helm

El chart oficial se encuentra en: [port-labs/helm-charts](https://github.com/port-labs/helm-charts/tree/main/charts/port-k8s-exporter)

#### Configuración de Webhook en Helm values.yaml

Para usar el modo WEBHOOK con Advanced Settings, crea un archivo `values.yaml`:

```yaml
# values.yaml para modo WEBHOOK
eventListener:
  type: "WEBHOOK"

stateKey: "my-cluster-state"
clusterName: "production-cluster"

# Webhook Configuration
webhook:
  url: "https://ingest.getport.io/your-webhook-endpoint"
  batchSize: 50
  batchTimeout: 5  # segundos

  # Advanced Settings (Security)
  secret: "your-hmac-secret"           # Secreto para firma HMAC
  signatureHeaderName: "X-Port-Signature"  # Nombre del header
  signatureAlgorithm: "sha256"         # sha256, sha1, sha512
  signaturePrefix: "sha256="           # Prefijo del valor de firma
  requestIdentifier: ""                # Path JQ para request ID (opcional)

# Config de recursos K8s a exportar
configMap:
  config: |
    resources:
      - kind: apps/v1/deployments
        port:
          entity:
            mappings:
              - identifier: .metadata.name + "-" + .metadata.namespace
                blueprint: '"deployment"'
```

Instalar con el archivo values:

```bash
helm install port-k8s-exporter port-labs/port-k8s-exporter -f values.yaml
```

> **Importante**: Los valores de `webhook.secret`, `webhook.signatureHeaderName`, `webhook.signatureAlgorithm` y `webhook.signaturePrefix` deben coincidir exactamente con los configurados en Port UI (Settings → Advanced Settings del webhook).

### Desde Código Fuente

```bash
# Clonar el repositorio
git clone https://github.com/port-labs/port-k8s-exporter.git
cd port-k8s-exporter

# Compilar
go build -o port-k8s-exporter .

# Ejecutar (requiere kubeconfig)
./port-k8s-exporter \
  --config /path/to/config.yaml \
  --event-listener-type POLLING \
  --port-client-id YOUR_CLIENT_ID \
  --port-client-secret YOUR_CLIENT_SECRET
```

## Configuración

### Variables de Entorno

| Variable | Descripción | Requerido | Default |
|----------|-------------|-----------|---------|
| `PORT_CLIENT_ID` | Client ID de Port | Solo POLLING/KAFKA | - |
| `PORT_CLIENT_SECRET` | Client Secret de Port | Solo POLLING/KAFKA | - |
| `PORT_BASE_URL` | URL base de la API de Port | No | `https://api.getport.io` |
| `STATE_KEY` | Clave única para identificar el estado | Sí | - |
| `EVENT_LISTENER_TYPE` | Tipo de event listener | No | `POLLING` |
| `RESYNC_INTERVAL` | Intervalo de resync en minutos | No | `1440` (24h) |
| `DELETE_DEPENDENTS` | Eliminar entidades dependientes | No | `false` |
| `CREATE_MISSING_RELATED_ENTITIES` | Crear entidades relacionadas faltantes | No | `false` |
| `WEBHOOK_URL` | URL del endpoint de ingest (WEBHOOK) | Solo WEBHOOK | `https://ingest.getport.io` |
| `WEBHOOK_BATCH_SIZE` | Tamaño del batch (WEBHOOK) | No | `100` |
| `WEBHOOK_BATCH_TIMEOUT` | Timeout del batch (WEBHOOK) | No | `5s` |
| `CLUSTER_NAME` | Nombre del cluster (WEBHOOK) | Solo WEBHOOK | - |
| `WEBHOOK_SECRET` | Secreto HMAC para firma (WEBHOOK) | No | - |
| `WEBHOOK_SIGNATURE_HEADER` | Nombre del header de firma | No | `X-Port-Signature` |
| `WEBHOOK_SIGNATURE_ALGORITHM` | Algoritmo de firma | No | `sha256` |
| `WEBHOOK_SIGNATURE_PREFIX` | Prefijo del valor de firma | No | - |
| `WEBHOOK_REQUEST_IDENTIFIER` | Path JQ para request ID | No | - |

### Flags de Línea de Comandos

```bash
./port-k8s-exporter --help

Flags:
  --config string              Path to config file (default "config.yaml")
  --port-client-id string      Port client ID
  --port-client-secret string  Port client secret
  --port-base-url string       Port API base URL (default "https://api.getport.io")
  --state-key string           Unique state key for this exporter instance
  --event-listener-type string Event listener type: POLLING, KAFKA, or WEBHOOK (default "POLLING")
  --resync-interval int        Resync interval in minutes (default 1440)
  --delete-dependents          Delete dependent entities on parent deletion
  --create-missing-related     Create missing related entities
  --webhook-url string         Webhook ingest URL (default "https://ingest.getport.io")
  --cluster-name string        Cluster name for webhook mode
  
  # Webhook Security (Advanced Settings)
  --webhook-secret string           Secret for HMAC signature verification
  --webhook-signature-header string Header name for signature (default "X-Port-Signature")
  --webhook-signature-algorithm string Signature algorithm: sha256, sha1, sha512 (default "sha256")
  --webhook-signature-prefix string Prefix for signature value (e.g., "sha256=")
  --webhook-request-identifier string JQ path to extract request identifier
```

### Archivo de Configuración (config.yaml)

```yaml
# Recursos a exportar
resources:
  # Deployments
  - kind: apps/v1/deployments
    selector:
      query: .metadata.namespace | startswith("kube") | not
    port:
      entity:
        mappings:
          - identifier: .metadata.name + "-" + .metadata.namespace
            title: .metadata.name
            blueprint: '"workload"'
            properties:
              namespace: .metadata.namespace
              replicas: .spec.replicas
              availableReplicas: .status.availableReplicas
              images: '[.spec.template.spec.containers[].image]'
              createdAt: .metadata.creationTimestamp
            relations:
              namespace: .metadata.namespace

  # Services
  - kind: v1/services
    selector:
      query: .metadata.namespace | startswith("kube") | not
    port:
      entity:
        mappings:
          - identifier: .metadata.name + "-" + .metadata.namespace
            title: .metadata.name
            blueprint: '"service"'
            properties:
              type: .spec.type
              clusterIP: .spec.clusterIP
              ports: .spec.ports

  # Namespaces
  - kind: v1/namespaces
    port:
      entity:
        mappings:
          - identifier: .metadata.name
            title: .metadata.name
            blueprint: '"namespace"'
            properties:
              createdAt: .metadata.creationTimestamp
              labels: .metadata.labels

  # ConfigMaps (ejemplo con selector)
  - kind: v1/configmaps
    selector:
      query: '.metadata.labels["app.kubernetes.io/managed-by"] == "Helm"'
    port:
      entity:
        mappings:
          - identifier: .metadata.name + "-" + .metadata.namespace
            blueprint: '"configmap"'
            properties:
              namespace: .metadata.namespace
              keys: '[.data | keys[]]'

  # CRDs personalizados
  - kind: mygroup.io/v1/myresources
    port:
      entity:
        mappings:
          - identifier: .metadata.name
            blueprint: '"myresource"'
            properties:
              status: .status.phase
              spec: .spec
```

### Expresiones JQ

Las expresiones JQ permiten transformar cualquier campo del recurso K8s:

```yaml
# Ejemplos de expresiones JQ comunes
mappings:
  # Concatenación de strings
  identifier: .metadata.name + "-" + .metadata.namespace
  
  # Valor literal (usar comillas dobles dentro)
  blueprint: '"deployment"'
  
  # Condicionales
  properties:
    status: 'if .status.availableReplicas == .spec.replicas then "healthy" else "degraded" end'
  
  # Filtrado de arrays
  containers: '[.spec.template.spec.containers[] | {name: .name, image: .image}]'
  
  # Acceso seguro a campos opcionales
  annotations: '.metadata.annotations // {}'
  
  # Transformación de labels
  labels: '[.metadata.labels | to_entries[] | "\(.key)=\(.value)"]'
```

## Desarrollo Local

### Pre-requisitos

- Go 1.24+
- Docker (para builds de contenedor)
- kubectl configurado con acceso a un cluster K8s
- Cuenta de Port (para modos POLLING/KAFKA)

### Compilar

```bash
# Compilar binario local
go build -o port-k8s-exporter .

# Ejecutar tests
go test ./...

# Ejecutar tests con verbose
go test -v ./...

# Test de un paquete específico
go test -v ./pkg/k8s/...
```

### Ejecutar Localmente

```bash
# Con archivo de config
export PORT_CLIENT_ID="your-client-id"
export PORT_CLIENT_SECRET="your-client-secret"
export STATE_KEY="local-dev"

./port-k8s-exporter --config ./my-config.yaml

# Con modo WEBHOOK (sin credenciales)
export STATE_KEY="local-dev"
export CLUSTER_NAME="local-cluster"
export EVENT_LISTENER_TYPE="WEBHOOK"

./port-k8s-exporter --config ./my-config.yaml
```

### Estructura del Proyecto

```
port-k8s-exporter/
├── main.go                    # Entrypoint de la aplicación
├── version.go                 # Información de versión
├── go.mod                     # Dependencias Go
├── Dockerfile                 # Build multi-arch
├── assets/
│   └── defaults/              # Configuración por defecto
│       ├── appConfig.yaml     # Config de aplicación
│       ├── blueprints.json    # Blueprints por defecto
│       ├── pages.json         # Pages por defecto
│       └── scorecards.json    # Scorecards por defecto
├── pkg/
│   ├── config/                # Configuración
│   │   ├── config.go          # Carga de configuración
│   │   ├── models.go          # Estructuras de datos
│   │   └── utils.go           # Utilidades
│   ├── crd/                   # Manejo de CRDs
│   ├── defaults/              # Aplicación de defaults
│   ├── event_handler/         # Event listeners
│   │   ├── event_listener_factory.go
│   │   ├── consumer/          # KAFKA listener
│   │   ├── polling/           # POLLING listener
│   │   └── webhook/           # WEBHOOK listener
│   ├── goutils/               # Utilidades generales
│   ├── handlers/              # Orquestación de controladores
│   ├── jq/                    # Parser JQ
│   ├── k8s/                   # Cliente y controlador K8s
│   ├── logger/                # Configuración de logging
│   ├── metrics/               # Métricas Prometheus
│   ├── parsers/               # Parsers (sensitive data)
│   ├── port/                  # Cliente de Port
│   │   ├── cli/               # HTTP client para API
│   │   ├── webhook/           # HTTP client para ingest
│   │   ├── blueprint/
│   │   ├── entity/
│   │   └── ...
│   └── signal/                # Manejo de señales
└── test_utils/                # Utilidades de testing
```

## Observabilidad

### Métricas Prometheus

El exporter expone métricas en el endpoint `/metrics`:

```
# Métricas disponibles
port_k8s_exporter_entities_synced_total{blueprint="deployment",action="upsert"} 150
port_k8s_exporter_entities_synced_total{blueprint="deployment",action="delete"} 10
port_k8s_exporter_sync_duration_seconds{blueprint="deployment"} 0.523
port_k8s_exporter_errors_total{type="api_error"} 2
port_k8s_exporter_resync_total 5
port_k8s_exporter_webhook_batch_size{} 100
port_k8s_exporter_webhook_requests_total{status="success"} 50
```

### Logging

El exporter usa `zap` para logging estructurado:

```bash
# Nivel de log configurable
export LOG_LEVEL=debug  # debug, info, warn, error

# Ejemplo de logs
{"level":"info","ts":"2024-01-15T10:30:00Z","msg":"Starting Port K8s Exporter","version":"0.2.0"}
{"level":"info","ts":"2024-01-15T10:30:01Z","msg":"Initialized controller","kind":"apps/v1/deployments"}
{"level":"debug","ts":"2024-01-15T10:30:02Z","msg":"Processing event","kind":"Deployment","name":"nginx","namespace":"default","action":"upsert"}
```

### Health Checks

```bash
# Liveness probe
GET /healthz

# Readiness probe  
GET /readyz
```

## Troubleshooting

### Problemas Comunes

#### 1. Error de autenticación con Port

```
Error: failed to authenticate with Port API
```

**Solución**: Verificar `PORT_CLIENT_ID` y `PORT_CLIENT_SECRET` son correctos.

#### 2. No se detectan cambios en recursos

**Posibles causas:**
- El selector JQ está filtrando los recursos
- El namespace no está incluido en la configuración
- Permisos RBAC insuficientes

**Verificar:**
```bash
# Ver logs del exporter
kubectl logs -f deployment/port-k8s-exporter

# Verificar permisos
kubectl auth can-i list deployments --as=system:serviceaccount:port:port-k8s-exporter
```

#### 3. Error en expresiones JQ

```
Error: jq parse error: ...
```

**Solución**: Validar la expresión JQ localmente:
```bash
kubectl get deployment nginx -o json | jq '.metadata.name + "-" + .metadata.namespace'
```

#### 4. Timeout en modo WEBHOOK

```
Error: webhook request timeout
```

**Solución**: Ajustar `WEBHOOK_BATCH_SIZE` y `WEBHOOK_BATCH_TIMEOUT`:
```yaml
env:
  - name: WEBHOOK_BATCH_SIZE
    value: "50"
  - name: WEBHOOK_BATCH_TIMEOUT
    value: "10s"
```

#### 5. Entidades no aparecen en Port

**Verificar:**
1. El blueprint existe en Port
2. Los campos requeridos están mapeados
3. No hay errores de validación en los logs

### Debug Mode

```bash
# Habilitar logs de debug
export LOG_LEVEL=debug

# Ver requests HTTP (solo desarrollo)
export HTTP_DEBUG=true
```

## Documentación Adicional

- [Port Docs](https://docs.getport.io) - Documentación oficial de Port
- [Helm Chart](https://github.com/port-labs/helm-charts/tree/main/charts/port-k8s-exporter) - Chart de Helm para despliegue
- [Port Webhook Ingest](https://docs.getport.io/build-your-software-catalog/sync-data-to-catalog/webhook/) - Documentación de webhooks

## Contribuir

1. Fork el repositorio
2. Crea una rama para tu feature (`git checkout -b feature/amazing-feature`)
3. Commit tus cambios (`git commit -m 'Add amazing feature'`)
4. Push a la rama (`git push origin feature/amazing-feature`)
5. Abre un Pull Request

## Licencia

Este proyecto está bajo la licencia Apache 2.0 - ver el archivo [LICENSE](LICENSE) para más detalles.
