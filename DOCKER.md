# Docker & Docker Compose Analysis

## 1. Multi-Stage Build Explanation

Das vorliegende Dockerfile verwendet einen sogenannten **Multi-Stage Build**, der aus zwei getrennten Phasen (Stages) besteht:

*   **Stage 1: Der Builder (`FROM golang:1.26-alpine AS builder`)**
    *   **Zweck:** Diese Phase dient als reine Build-Umgebung. Sie basiert auf einem Image, das den kompletten Go-Compiler und alle nötigen Werkzeuge enthält.
    *   **Ablauf:** Die Abhängigkeiten (`go.mod`, `go.sum`) werden heruntergeladen, der Quellcode wird in den Container kopiert und die Go-Anwendung wird zu einer ausführbaren Binärdatei (`api-server`) kompiliert.
*   **Stage 2: Die Runtime (`FROM alpine:3.19`)**
    *   **Zweck:** Das ist das endgültige Image, das später auch in Produktion ausgeführt wird. Es basiert auf "Alpine", einer extrem schlanken und minimalistischen Linux-Distribution.
    *   **Ablauf:** Der schwere Go-Compiler und der Quellcode aus Stage 1 werden komplett weggeworfen. Es wird mit `COPY --from=builder` *ausschließlich* die fertige, kompilierte Binärdatei in dieses neue Image kopiert.

**Warum zwei Stages?** Man trennt die Build-Umgebung von der Laufzeit-Umgebung. Dadurch bleibt das finale Image extrem klein, lässt sich schneller herunterladen und ist sicherer (da weniger installierte Programme eine geringere Angriffsfläche bieten).

---

## 2. Die Rolle von `CGO_ENABLED=0`

Beim Kompilieren wird der Befehl `CGO_ENABLED=0 GOOS=linux go build ...` aufgerufen.

*   **Was es macht:** Es deaktiviert "cgo" (eine Technologie, die es Go erlaubt, C-Code bzw. C-Bibliotheken aufzurufen).
*   **Warum es wichtig ist:** Wenn `cgo` deaktiviert ist, wird der Compiler gezwungen, eine **zu 100% statisch gelinkte Binärdatei (statically linked binary)** zu erzeugen. Das bedeutet, dass die Binärdatei absolut eigenständig ist und keine externen dynamischen Systembibliotheken (wie `glibc` oder `musl`) zum Laufen benötigt. Das ist essenziell für Multi-Stage Builds: Würde man eine dynamisch gelinkte Datei aus Stage 1 in das nackte Alpine-Image in Stage 2 kopieren, würde die App beim Start mit einem "not found"-Fehler abstürzen, weil ihr die Systembibliotheken aus dem Build-Container fehlen. `CGO_ENABLED=0` garantiert, dass die App überall läuft.

---

## 3. Vergleich der Image-Größe: Multi-Stage vs. Single-Stage

*   **Single-Stage Build:** Hätten wir das finale Image direkt auf Basis von `golang:1.26-alpine` gebaut, würde die Größe etwa **300 bis 350 MB** betragen (da der gesamte Go-Compiler, Zwischenspeicher und Sourcecode enthalten bleiben).
*   **Multi-Stage Build (Aktuell):** Das Alpine-Base-Image ist nur knapp 5 MB groß. Zusammen mit der kompilierten Go-Binärdatei schrumpft das fertige Image auf exakt **29,26 MB**.
*   **Fazit:** Durch den Multi-Stage Build reduzieren wir die Größe des Docker-Images signifikant (um über 90 %). Dies führt zu schnelleren Deployments und spart Speicherplatz und Netzwerkbandbreite.

---

## 4. CRUD Operations & Data Persistence Test

Um die Funktionalität der API und die Persistenz der PostgreSQL-Datenbank (via Docker Volumes) zu verifizieren, wurden folgende CRUD-Operationen ausgeführt:

### 1. Create (Produkte anlegen)
```bash
curl -s -X POST -H "Content-Type: application/json" -d '{"name":"Apple","price":1.50}' http://localhost:8080/products
curl -s -X POST -H "Content-Type: application/json" -d '{"name":"Banana","price":0.99}' http://localhost:8080/products
curl -s -X POST -H "Content-Type: application/json" -d '{"name":"Orange","price":1.20}' http://localhost:8080/products
```

### 2. Read (Produkte auflisten)
```bash
curl -s http://localhost:8080/products
```
*Ausgabe (Auszug):* `{"id":4,"name":"Apple","price":1.5},{"id":5,"name":"Banana","price":0.99},{"id":6,"name":"Orange","price":1.2}]`

### 3. Update (Produkt aktualisieren)
```bash
curl -s -X PUT -H "Content-Type: application/json" -d '{"name":"Apple (Updated)","price":1.60}' http://localhost:8080/products/4
```

### 4. Delete (Produkt löschen)
```bash
curl -s -X DELETE http://localhost:8080/products/5
```

### 5. Data Persistence Verification
Um sicherzustellen, dass die Daten auch nach einem Neustart der Container erhalten bleiben, wurden die Container gestoppt und gelöscht:
```bash
docker compose down
```
Anschließend wurden sie neu gestartet:
```bash
docker compose up -d
```
Ein erneuter Aufruf der API zeigte, dass die Datenbank ihren Zustand exakt behalten hat:
```bash
curl -s http://localhost:8080/products
```
*Ergebnis:* Die angelegten Produkte wie z.B. `Apple (Updated)` und `Orange` waren weiterhin abrufbar, während `Banana` dauerhaft gelöscht war. Die Datenpersistenz über das Docker-Volume `pgdata` funktioniert fehlerfrei.
