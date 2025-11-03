# LuCICodex - Asistente de Lenguaje Natural para OpenWrt

**Controla tu router OpenWrt con comandos en español simple**

Autor: AZ <Aezi.zhu@icloud.com>

<p align="center">
  <a href="#"><img alt="Build" src="https://img.shields.io/badge/build-passing-brightgreen"></a>
  <a href="#license"><img alt="License" src="https://img.shields.io/badge/license-Dual-blue"></a>
  <a href="#"><img alt="Go Version" src="https://img.shields.io/badge/Go-1.21+-1f425f"></a>
  <a href="#"><img alt="OpenWrt" src="https://img.shields.io/badge/OpenWrt-supported-00a0e9"></a>
  <a href="https://github.com/aezizhu/LuciCodex/actions/workflows/build.yml"><img alt="CI" src="https://github.com/aezizhu/LuciCodex/actions/workflows/build.yml/badge.svg"></a>
</p>

**[English](README.md)** | **[中文](README.zh-CN.md)** | **[Español](README.es.md)**

---

## ¿Qué es LuCICodex?

**LuCICodex** es un asistente inteligente que te permite administrar tu router OpenWrt usando lenguaje natural en lugar de memorizar comandos complejos. Simplemente dile a LuCICodex lo que quieres hacer en español simple, y traducirá tu solicitud en comandos seguros y auditados que puedes revisar antes de ejecutarlos.

**Ejemplo:** En lugar de recordar `uci set wireless.radio0.disabled=0 && uci commit wireless && wifi reload`, solo di: *"enciende el wifi"*

Nota: Este proyecto se llamaba anteriormente "g". Todos los alias heredados han sido eliminados; usa `lucicodex` exclusivamente.

---

## Tabla de Contenidos

- [¿Por Qué Usar LuCICodex?](#por-qué-usar-lucicodex)
- [Primeros Pasos](#primeros-pasos)
  - [Requisitos Previos](#requisitos-previos)
  - [Instalación en OpenWrt](#instalación-en-openwrt)
  - [Obtener tu Clave API](#obtener-tu-clave-api)
- [Usando LuCICodex en tu Router](#usando-lucicodex-en-tu-router)
  - [Método 1: Interfaz Web (Recomendado)](#método-1-interfaz-web-recomendado)
  - [Método 2: Línea de Comandos (SSH)](#método-2-línea-de-comandos-ssh)
- [Guía de Configuración](#guía-de-configuración)
  - [Eligiendo tu Proveedor de IA](#eligiendo-tu-proveedor-de-ia)
  - [Configuración vía Interfaz Web](#configuración-vía-interfaz-web)
  - [Configuración vía Línea de Comandos](#configuración-vía-línea-de-comandos)
- [Casos de Uso Comunes](#casos-de-uso-comunes)
- [Características de Seguridad](#características-de-seguridad)
- [Solución de Problemas](#solución-de-problemas)
- [Uso Avanzado](#uso-avanzado)
- [Licencia](#licencia)
- [Soporte](#soporte)

---

## ¿Por Qué Usar LuCICodex?

### Para Usuarios Domésticos
- **Sin memorización de comandos**: Administra tu router en español simple
- **Seguro por defecto**: Todos los comandos se revisan antes de ejecutarse
- **Interfaz web fácil**: No es necesario usar SSH en tu router
- **Aprende mientras usas**: Ve los comandos reales que genera LuciCodex

### Para Usuarios Avanzados
- **Administración más rápida**: El lenguaje natural es más rápido que buscar sintaxis
- **Seguridad basada en políticas**: Personaliza qué comandos están permitidos
- **Múltiples proveedores de IA**: Elige entre Gemini, OpenAI o Anthropic
- **Registro de auditoría**: Registro completo de todas las operaciones

---

## Primeros Pasos

### Requisitos Previos

Antes de instalar LuciCodex, necesitas:

1. **Un router OpenWrt** (versión 21.02 o posterior recomendada)
2. **Conexión a Internet** en tu router
3. Al menos **10MB de espacio libre** de almacenamiento
4. **Una clave API** de uno de estos proveedores:
   - Google Gemini (recomendado para principiantes - plan gratuito disponible)
   - OpenAI (GPT-4/GPT-3.5)
   - Anthropic (Claude)

### Instalación en OpenWrt

#### Paso 1: Descargar el Paquete

Conéctate a tu router por SSH y descarga el paquete LuCICodex para tu arquitectura:

```bash
# Para routers MIPS (más común)
cd /tmp
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-mips.ipk

# Para routers ARM
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-arm.ipk

# Para routers x86_64
wget https://github.com/aezizhu/LuciCodex/releases/latest/download/lucicodex-amd64.ipk
```

#### Paso 2: Instalar el Paquete

```bash
opkg update
opkg install /tmp/lucicodex-*.ipk
```

#### Paso 3: Instalar la Interfaz Web (Opcional pero Recomendado)

```bash
opkg install luci-app-lucicodex
```

#### Paso 4: Verificar la Instalación

```bash
lucicodex -version
```

Deberías ver: `LuciCodex version 0.3.0`

### Obtener tu Clave API

#### Opción 1: Google Gemini (Recomendado para Principiantes)

1. Visita https://makersuite.google.com/app/apikey
2. Haz clic en "Create API Key"
3. Copia tu clave API (comienza con `AIza...`)
4. **Plan gratuito**: 60 solicitudes por minuto

#### Opción 2: OpenAI

1. Visita https://platform.openai.com/api-keys
2. Haz clic en "Create new secret key"
3. Copia tu clave API (comienza con `sk-...`)
4. **Nota**: Requiere método de pago registrado

#### Opción 3: Anthropic

1. Visita https://console.anthropic.com/settings/keys
2. Haz clic en "Create Key"
3. Copia tu clave API (comienza con `sk-ant-...`)
4. **Nota**: Requiere método de pago registrado

---

## Usando LuCICodex en tu Router

### Método 1: Interfaz Web (Recomendado)

Esta es la forma más fácil de usar LuciCodex, especialmente si no te sientes cómodo con la línea de comandos.

#### Paso 1: Acceder a la Interfaz Web

1. Abre la interfaz web de tu router (normalmente http://192.168.1.1)
2. Inicia sesión con tus credenciales de administrador
3. Navega a **Sistema → LuCICodex**

#### Paso 2: Configurar tu Clave API

1. Haz clic en la pestaña **Configuración**
2. Selecciona tu proveedor de IA (Gemini, OpenAI o Anthropic)
3. Ingresa tu clave API en el campo correspondiente
4. Haz clic en **Guardar y Aplicar**

#### Paso 3: Usar el Asistente

1. Haz clic en la pestaña **Ejecutar**
2. Escribe tu solicitud en español simple, por ejemplo:
   - "Muéstrame la configuración de red actual"
   - "Reinicia el wifi"
   - "Abre el puerto 8080 para mi servidor web"
3. Haz clic en **Generar Plan**
4. Revisa los comandos que sugiere LuciCodex
5. Si se ven correctos, haz clic en **Ejecutar Comandos**

**¡Eso es todo!** Ahora estás usando lenguaje natural para controlar tu router.

### Método 2: Línea de Comandos (SSH)

Si prefieres usar SSH o quieres automatizar tareas, puedes usar LuciCodex desde la línea de comandos.

#### Paso 1: Configurar tu Clave API

```bash
# Configura tu clave API de Gemini
uci set lucicodex.@api[0].provider='gemini'
uci set lucicodex.@api[0].key='TU-CLAVE-API-AQUI'
uci commit lucicodex
```

#### Paso 2: Probar con una Ejecución de Prueba

```bash
lucicodex "muéstrame el estado del wifi"
```

Esto mostrará qué comandos ejecutaría LuciCodex, pero no los ejecutará todavía.

#### Paso 3: Ejecutar Comandos

Si los comandos se ven correctos, ejecuta con aprobación:

```bash
lucicodex -approve "reinicia el wifi"
```

O usa el modo interactivo para confirmar cada comando:

```bash
lucicodex -confirm-each "actualiza la lista de paquetes e instala htop"
```

---

## Guía de Configuración

### Eligiendo tu Proveedor de IA

LuciCodex soporta múltiples proveedores de IA. Así es como elegir:

| Proveedor | Mejor Para | Costo | Velocidad | Clave API Requerida |
|-----------|------------|-------|-----------|---------------------|
| **Gemini** | Principiantes, usuarios domésticos | Plan gratuito disponible | Rápido | GEMINI_API_KEY o lucicodex.@api[0].key |
| **OpenAI** | Usuarios avanzados, tareas complejas | Pago por uso | Muy rápido | OPENAI_API_KEY o lucicodex.@api[0].openai_key |
| **Anthropic** | Usuarios conscientes de la privacidad | Pago por uso | Rápido | ANTHROPIC_API_KEY o lucicodex.@api[0].anthropic_key |
| **Gemini CLI** | Uso offline/local | Gratis (local) | Varía | Ruta del binario gemini externo |

**Nota:** Cada proveedor requiere su propia clave API específica. Solo necesitas configurar la clave del proveedor que estés usando.

### Configuración vía Interfaz Web

1. Ve a **Sistema → LuCICodex → Configuración**
2. Configura estos ajustes:

**Configuración de API:**
- **Proveedor**: Elige tu proveedor de IA
- **Clave API**: Ingresa tu clave (almacenada de forma segura)
- **Modelo**: Deja vacío para usar el predeterminado, o especifica (ej., `gemini-1.5-flash`, `gpt-4`, `claude-3-sonnet`)
- **Endpoint**: Deja el predeterminado a menos que uses un endpoint personalizado

**Configuración de Seguridad:**
- **Ejecución de Prueba por Defecto**: Mantén habilitado (recomendado) - muestra comandos antes de ejecutar
- **Confirmar Cada Comando**: Habilita para seguridad extra
- **Tiempo de Espera de Comando**: Cuánto esperar por cada comando (predeterminado: 30 segundos)
- **Máximo de Comandos**: Comandos máximos por solicitud (predeterminado: 10)
- **Archivo de Registro**: Dónde guardar registros de ejecución (predeterminado: `/tmp/lucicodex.log`)

3. Haz clic en **Guardar y Aplicar**

### Configuración vía Línea de Comandos

Todos los ajustes se almacenan en `/etc/config/lucicodex` usando el sistema UCI de OpenWrt:

```bash
# Configurar Gemini
uci set lucicodex.@api[0].provider='gemini'
uci set lucicodex.@api[0].key='TU-CLAVE-GEMINI'
uci set lucicodex.@api[0].model='gemini-1.5-flash'

# Configurar OpenAI
uci set lucicodex.@api[0].provider='openai'
uci set lucicodex.@api[0].openai_key='TU-CLAVE-OPENAI'
uci set lucicodex.@api[0].model='gpt-4'

# Configurar Anthropic
uci set lucicodex.@api[0].provider='anthropic'
uci set lucicodex.@api[0].anthropic_key='TU-CLAVE-ANTHROPIC'
uci set lucicodex.@api[0].model='claude-3-sonnet-20240229'

# Configuración de seguridad
uci set lucicodex.@settings[0].dry_run='1'          # 1=habilitado, 0=deshabilitado
uci set lucicodex.@settings[0].confirm_each='0'     # 1=confirmar cada uno, 0=confirmar una vez
uci set lucicodex.@settings[0].timeout='30'         # segundos
uci set lucicodex.@settings[0].max_commands='10'    # comandos máximos por solicitud

# Aplicar cambios
uci commit lucicodex
```

---

## Casos de Uso Comunes

### Gestión de Red

```bash
# Verificar estado de red
lucicodex "muéstrame todas las interfaces de red y su estado"

# Reiniciar red
lucicodex -approve "reinicia la red"

# Configurar IP estática
lucicodex "configura la interfaz lan con IP estática 192.168.1.1"
```

### Gestión de WiFi

```bash
# Verificar estado de WiFi
lucicodex "muéstrame el estado del wifi"

# Cambiar contraseña de WiFi
lucicodex "cambia la contraseña del wifi a MiNuevaContraseña123"

# Habilitar/deshabilitar WiFi
lucicodex -approve "apaga el wifi"
lucicodex -approve "enciende el wifi"

# Reiniciar WiFi
lucicodex -approve "reinicia el wifi"
```

### Gestión de Firewall

```bash
# Verificar reglas de firewall
lucicodex "muéstrame las reglas del firewall actuales"

# Abrir un puerto
lucicodex "abre el puerto 8080 para tráfico tcp desde lan"

# Bloquear una IP
lucicodex "bloquea la dirección IP 192.168.1.100"
```

### Gestión de Paquetes

```bash
# Actualizar lista de paquetes
lucicodex "actualiza la lista de paquetes"

# Instalar un paquete
lucicodex "instala el paquete htop"

# Listar paquetes instalados
lucicodex "muéstrame todos los paquetes instalados"
```

### Monitoreo del Sistema

```bash
# Verificar estado del sistema
lucicodex "muéstrame información del sistema y tiempo de actividad"

# Verificar uso de memoria
lucicodex "muéstrame el uso de memoria"

# Verificar espacio en disco
lucicodex "muéstrame el uso de espacio en disco"

# Ver registros del sistema
lucicodex "muéstrame las últimas 20 líneas del registro del sistema"
```

### Diagnósticos

```bash
# Prueba de ping
lucicodex "haz ping a google.com 5 veces"

# Prueba de DNS
lucicodex "verifica si el dns está funcionando"

# Verificar conectividad a internet
lucicodex "prueba la conexión a internet"
```

---

## Características de Seguridad

LuCICodex está diseñado con la seguridad como máxima prioridad:

### 1. Modo de Ejecución de Prueba (Predeterminado)
Por defecto, LuciCodex te muestra lo que haría sin hacerlo realmente. Debes aprobar explícitamente la ejecución.

### 2. Revisión de Comandos
Cada comando se te muestra antes de ejecutarse. Puedes ver exactamente qué se ejecutará en tu sistema.

### 3. Motor de Políticas
LuCICodex tiene reglas integradas sobre qué comandos están permitidos:

**Permitidos por defecto:**
- `uci` (configuración)
- `ubus` (bus del sistema)
- `fw4` (firewall)
- `opkg` (gestor de paquetes)
- `ip`, `ifconfig` (información de red)
- `cat`, `grep`, `tail` (leer archivos)
- `logread`, `dmesg` (registros)

**Bloqueados por defecto:**
- `rm -rf /` (eliminaciones peligrosas)
- `mkfs` (formateo de sistema de archivos)
- `dd` (operaciones de disco)
- Bombas fork y otros patrones maliciosos

### 4. Sin Ejecución de Shell
LuCICodex nunca usa expansión de shell o tuberías. Los comandos se ejecutan directamente con argumentos exactos, previniendo ataques de inyección.

### 5. Bloqueo de Ejecución
Solo un comando de LuciCodex puede ejecutarse a la vez, previniendo conflictos y condiciones de carrera. El CLI usa un archivo de bloqueo en `/var/lock/lucicodex.lock` (o `/tmp/lucicodex.lock` como respaldo) para asegurar ejecución exclusiva.

### 6. Tiempos de Espera
Cada comando tiene un tiempo de espera (predeterminado 30 segundos) para prevenir bloqueos.

### 7. Registro de Auditoría
Todos los comandos y sus resultados se registran en `/tmp/lucicodex.log` para revisión.

---

## Solución de Problemas

### "API key not configured" (Clave API no configurada)

**Solución:** Asegúrate de haber configurado tu clave API:

```bash
# Vía UCI
uci set lucicodex.@api[0].key='TU-CLAVE-AQUI'
uci commit lucicodex

# O vía variable de entorno
export GEMINI_API_KEY='TU-CLAVE-AQUI'
```

### "execution in progress" (ejecución en progreso)

**Solución:** Otro comando de LuciCodex está ejecutándose. Espera a que termine, o elimina el archivo de bloqueo obsoleto:

```bash
rm /var/lock/lucicodex.lock
# o si usas la ubicación de respaldo:
rm /tmp/lucicodex.lock
```

### "command not found: lucicodex" (comando no encontrado: lucicodex)

**Solución:** Asegúrate de que lucicodex esté instalado y en tu PATH:

```bash
which lucicodex
# Debería mostrar: /usr/bin/lucicodex

# Si no se encuentra, reinstala:
opkg update
opkg install lucicodex
```

### Los comandos no se están ejecutando

**Solución:** Asegúrate de no estar en modo de ejecución de prueba:

```bash
# Usa la bandera -approve
lucicodex -approve "tu comando aquí"

# O deshabilita la ejecución de prueba en la configuración
uci set lucicodex.@settings[0].dry_run='0'
uci commit lucicodex
```

### "prompt too long (max 4096 chars)" (solicitud demasiado larga)

**Solución:** Tu solicitud es demasiado larga. Divídela en solicitudes más pequeñas o sé más conciso.

### La interfaz web no aparece

**Solución:** Asegúrate de que luci-app-lucicodex esté instalado:

```bash
opkg update
opkg install luci-app-lucicodex
/etc/init.d/uhttpd restart
```

Luego limpia la caché de tu navegador y recarga.

---

## Uso Avanzado

### Modo Interactivo (REPL)

Inicia una sesión interactiva donde puedes tener una conversación con LuciCodex:

```bash
lucicodex -interactive
```

### Salida JSON

Obtén salida estructurada para scripting:

```bash
lucicodex -json "muestra el estado de red" | jq .
```

### Archivo de Configuración Personalizado

Usa un archivo de configuración personalizado en lugar de UCI:

```bash
lucicodex -config /etc/lucicodex/custom-config.json "tu comando"
```

### Variables de Entorno

Sobrescribe configuraciones con variables de entorno:

```bash
export GEMINI_API_KEY='tu-clave'
export LUCICODEX_PROVIDER='gemini'
export LUCICODEX_MODEL='gemini-1.5-flash'
lucicodex "tu comando"
```

### Banderas de Línea de Comandos

```bash
lucicodex -help
```

Banderas disponibles:
- `-approve`: Auto-aprobar plan sin confirmación
- `-dry-run`: Solo mostrar plan, no ejecutar (predeterminado: true)
- `-confirm-each`: Confirmar cada comando individualmente
- `-json`: Emitir salida en formato JSON
- `-interactive`: Iniciar modo REPL interactivo
- `-timeout=30`: Establecer tiempo de espera de comando en segundos
- `-max-commands=10`: Establecer máximo de comandos por solicitud
- `-model=name`: Sobrescribir nombre del modelo
- `-config=path`: Usar archivo de configuración personalizado
- `-log-file=path`: Establecer ruta del archivo de registro
- `-facts=true`: Incluir información del entorno en el prompt (predeterminado: true)
- `-join-args`: Unir todos los argumentos en un solo prompt (experimental)
- `-version`: Mostrar versión

**Nota sobre manejo de prompts:** Por defecto, LuciCodex usa solo el primer argumento como prompt. Si necesitas pasar prompts de múltiples palabras sin comillas, usa la bandera `-join-args`:

```bash
# Comportamiento predeterminado (recomendado)
lucicodex "muestra el estado del wifi"

# Con bandera -join-args (experimental)
lucicodex -join-args muestra el estado del wifi
```

### Personalizando la Política

Edita la lista de permitidos y denegados en `/etc/config/lucicodex` o tu archivo de configuración:

```json
{
  "allowlist": [
    "^uci(\\s|$)",
    "^comando-personalizado(\\s|$)"
  ],
  "denylist": [
    "^comando-peligroso(\\s|$)"
  ]
}
```

---

## Licencia

**Licencia Dual:**

- **Gratis para uso individual/personal** - Usa LuciCodex en tu router doméstico sin costo
- **Uso comercial requiere licencia** - Contacta aezi.zhu@icloud.com para licencias comerciales

Ver archivo [LICENSE](LICENSE) para detalles completos.

---

## Soporte

### Obtener Ayuda

- **Documentación**: ¡La estás leyendo!
- **Problemas**: https://github.com/aezizhu/LuciCodex/issues
- **Discusiones**: https://github.com/aezizhu/LuciCodex/discussions

### Soporte Comercial

Para licencias comerciales, soporte empresarial o desarrollo personalizado:
- Email: Aezi.zhu@icloud.com
- Incluye "LuciCodex Commercial License" en el asunto

### Contribuir

¡Las contribuciones son bienvenidas! Por favor lee nuestras guías de contribución antes de enviar pull requests.

---

## Sobre Este Proyecto

**LuciCodex** fue creado para hacer la administración de routers OpenWrt accesible para todos, no solo para expertos en redes. Al combinar el poder de la IA moderna con controles de seguridad estrictos, LuciCodex te permite administrar tu router usando lenguaje natural mientras mantiene la seguridad y transparencia.

El proyecto se enfoca en OpenWrt primero, con un diseño agnóstico del proveedor y valores predeterminados de seguridad sólidos. Cada comando es auditado, cada acción es registrada, y tú siempre tienes el control.

---

**Hecho con ❤️ para la comunidad OpenWrt**
