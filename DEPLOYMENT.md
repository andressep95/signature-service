# Guía de Despliegue en Contabo VPS

## Pre-requisitos

- Docker instalado en tu máquina local
- Acceso SSH a tu VPS en Contabo
- Docker instalado en el VPS de Contabo
- Credenciales de AWS IAM configuradas

---

## Opción 1: Usando Docker Hub (Recomendada)

### Paso 1: Construir y Publicar la Imagen

```bash
# En tu máquina local, dentro del directorio del proyecto

# 1. Construir la imagen
docker build -t tu-usuario-dockerhub/signer-service:latest .

# 2. Login en Docker Hub
docker login

# 3. Subir la imagen
docker push tu-usuario-dockerhub/signer-service:latest
```

### Paso 2: Desplegar en Contabo

```bash
# Conectar a tu VPS
ssh usuario@tu-ip-contabo

# 1. Descargar la imagen
docker pull tu-usuario-dockerhub/signer-service:latest

# 2. Ejecutar el contenedor
docker run -d \
  --name signer-service \
  -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=tu-access-key \
  -e AWS_SECRET_ACCESS_KEY=tu-secret-key \
  -e S3_BUCKET_NAME=aws-muppet \
  -e COMPANY_PREFIX=addi \
  -e PRESIGNED_URL_EXPIRATION_MINUTES=15 \
  --restart unless-stopped \
  tu-usuario-dockerhub/signer-service:latest

# 3. Verificar que está corriendo
docker ps
docker logs signer-service

# 4. Probar el servicio
curl http://localhost:8080/health
```

---

## Opción 2: Transferir Imagen Directamente (Sin Docker Hub)

### Paso 1: Construir y Exportar

```bash
# En tu máquina local

# 1. Construir la imagen
docker build -t signer-service:latest .

# 2. Exportar a archivo tar
docker save -o signer-service.tar signer-service:latest

# 3. Transferir al VPS (puede tardar según el tamaño)
scp signer-service.tar usuario@tu-ip-contabo:/tmp/
```

### Paso 2: Importar y Ejecutar en Contabo

```bash
# En el VPS de Contabo

# 1. Importar la imagen
docker load -i /tmp/signer-service.tar

# 2. Ejecutar el contenedor (mismo comando que Opción 1)
docker run -d \
  --name signer-service \
  -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=tu-access-key \
  -e AWS_SECRET_ACCESS_KEY=tu-secret-key \
  -e S3_BUCKET_NAME=aws-muppet \
  -e COMPANY_PREFIX=addi \
  -e PRESIGNED_URL_EXPIRATION_MINUTES=15 \
  --restart unless-stopped \
  signer-service:latest

# 3. Limpiar archivo temporal
rm /tmp/signer-service.tar
```

---

## Opción 3: Construir Directamente en Contabo

```bash
# En el VPS de Contabo

# 1. Clonar el repositorio
git clone tu-repo-url
cd signer-service

# 2. Construir la imagen
docker build -t signer-service:latest .

# 3. Ejecutar (mismo comando que las opciones anteriores)
docker run -d \
  --name signer-service \
  -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=tu-access-key \
  -e AWS_SECRET_ACCESS_KEY=tu-secret-key \
  -e S3_BUCKET_NAME=aws-muppet \
  -e COMPANY_PREFIX=addi \
  -e PRESIGNED_URL_EXPIRATION_MINUTES=15 \
  --restart unless-stopped \
  signer-service:latest
```

---

## Gestión del Servicio

### Ver logs
```bash
docker logs signer-service
docker logs -f signer-service  # Modo seguimiento
```

### Reiniciar
```bash
docker restart signer-service
```

### Detener
```bash
docker stop signer-service
```

### Eliminar contenedor
```bash
docker stop signer-service
docker rm signer-service
```

### Actualizar a nueva versión
```bash
# 1. Detener y eliminar contenedor actual
docker stop signer-service
docker rm signer-service

# 2. Descargar nueva imagen (Opción Docker Hub)
docker pull tu-usuario-dockerhub/signer-service:latest

# 3. Ejecutar de nuevo (usar mismo comando docker run)
docker run -d ...
```

---

## Multi-Tenancy (Múltiples Empresas)

Para ejecutar múltiples instancias (una por empresa):

```bash
# Empresa 1: Addi
docker run -d \
  --name signer-service-addi \
  -p 8081:8080 \
  -e AWS_ACCESS_KEY_ID=credenciales-addi \
  -e AWS_SECRET_ACCESS_KEY=secret-addi \
  -e S3_BUCKET_NAME=aws-muppet \
  -e COMPANY_PREFIX=addi \
  -e PRESIGNED_URL_EXPIRATION_MINUTES=15 \
  --restart unless-stopped \
  signer-service:latest

# Empresa 2: Sourcing
docker run -d \
  --name signer-service-sourcing \
  -p 8082:8080 \
  -e AWS_ACCESS_KEY_ID=credenciales-sourcing \
  -e AWS_SECRET_ACCESS_KEY=secret-sourcing \
  -e S3_BUCKET_NAME=aws-muppet \
  -e COMPANY_PREFIX=sourcing \
  -e PRESIGNED_URL_EXPIRATION_MINUTES=15 \
  --restart unless-stopped \
  signer-service:latest
```

Cada instancia:
- Escucha en un puerto diferente (8081, 8082, etc.)
- Usa credenciales IAM propias
- Tiene su propio COMPANY_PREFIX

---

## Seguridad de Variables de Entorno

### Opción Segura: Archivo de Secrets

```bash
# 1. Crear archivo de variables (NUNCA subir a git)
cat > /etc/signer-service/secrets.env <<EOF
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
S3_BUCKET_NAME=aws-muppet
COMPANY_PREFIX=addi
PRESIGNED_URL_EXPIRATION_MINUTES=15
EOF

# 2. Proteger el archivo
chmod 600 /etc/signer-service/secrets.env

# 3. Ejecutar con --env-file
docker run -d \
  --name signer-service \
  -p 8080:8080 \
  --env-file /etc/signer-service/secrets.env \
  --restart unless-stopped \
  signer-service:latest
```

---

## Firewall (Importante)

Abrir el puerto en el firewall de Contabo:

```bash
# UFW (Ubuntu/Debian)
sudo ufw allow 8080/tcp
sudo ufw reload

# iptables
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
sudo iptables-save
```

---

## Verificación Post-Deployment

```bash
# 1. Health check
curl http://localhost:8080/health

# 2. Buscar objeto (debe retornar 200)
curl -X POST http://localhost:8080/api/v1/object/search \
  -H "Content-Type: application/json" \
  -d '{"filename": "test.tar.gz"}'

# 3. Generar presigned URL (debe retornar URL firmada)
curl -X POST http://localhost:8080/api/v1/presigned-url/upload \
  -H "Content-Type: application/json" \
  -d '{"filename": "test.tar.gz", "content_type": "application/gzip"}'
```

---

## Troubleshooting

### Contenedor no inicia
```bash
docker logs signer-service
# Verificar que todas las variables requeridas estén configuradas
```

### Error de conexión a AWS
```bash
# Verificar credenciales
docker exec signer-service env | grep AWS

# Verificar conectividad
docker exec signer-service wget -O- https://s3.us-east-1.amazonaws.com
```

### Puerto ya en uso
```bash
# Ver qué está usando el puerto
sudo lsof -i :8080
# O cambiar el puerto de exposición: -p 8081:8080
```

---

## Monitoreo

### Ver uso de recursos
```bash
docker stats signer-service
```

### Logs con rotación (evitar que crezcan indefinidamente)
```bash
docker run -d \
  --name signer-service \
  --log-driver json-file \
  --log-opt max-size=10m \
  --log-opt max-file=3 \
  -p 8080:8080 \
  --env-file /etc/signer-service/secrets.env \
  --restart unless-stopped \
  signer-service:latest
```
