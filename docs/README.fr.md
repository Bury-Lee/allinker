# allinker — Passerelle de Collaboration Inter-Agents

> Un point d'entrée de collaboration unifié pour différents logiciels d'IA Agent, permettant un travail collaboratif entre agents.

![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-green)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

[English](../README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [한국어](README.ko.md)

---

## Aperçu

allinker est une **passerelle de collaboration en CLI** conçue pour plusieurs outils d'IA Agent (tels que Cline, CodeX, agents personnalisés, etc.) travaillant dans le même répertoire de projet.

Lorsque plusieurs agents opèrent indépendamment dans le même projet, ils sont confrontés à :

- **Conflits de fichiers** — Plusieurs agents modifiant le même fichier simultanément
- **Silos d'information** — Aucune communication directe entre agents
- **Opérations introuvables** — Impossible d'auditer qui a fait quoi et quand

allinker résout ces problèmes avec **quatre primitives de collaboration** :

| Primitive | Problème résolu |
|-----------|----------------|
| **Verrouillage de fichiers** | Les agents acquièrent un verrou avant d'éditer un fichier pour éviter les conflits |
| **Messagerie** | Les agents s'envoient des messages avec des mentions `@` |
| **Surveillance de fichiers** | Les agents enregistrent des points de surveillance pour suivre la progression des pairs |
| **Gestion des comptes** | Signature d'identité + 3 niveaux de permissions + piste d'audit complète |

---

## Démarrage rapide

### Compilation

```bash
git clone <repo-url>
cd allinker
go build -o allinker.exe .
```

Des binaires pré-compilés sont également disponibles pour Windows (x64/x86), Linux (x64/ARM64) et macOS (Intel/ARM).

### Enregistrement des agents

```bash
./allinker register --name TRAE --role agent
./allinker register --name CodeX --role agent
./allinker register --name admin --role admin
```

### Verrouillage de fichiers

```bash
./allinker lock -f PLAN_001.md -t 30 --user TRAE    # Verrou bloquant (max 30s)
./allinker tryLock -f PLAN_001.md --user TRAE        # Tentative non bloquante
./allinker unlock -f PLAN_001.md --user TRAE         # Libérer le verrou
./allinker status -f PLAN_001.md                     # Voir l'état du verrou
./allinker status --all                              # Lister tous les verrous
```

### Messagerie

```bash
./allinker send --at CodeX --msg "Veuillez implémenter le module d'authentification" --user TRAE
./allinker send --at All --msg "Message général" --user TRAE
./allinker recv                                                   # Recevoir des messages
./allinker history --with CodeX --limit 10                        # Voir l'historique
```

### Surveillance de fichiers — Attente de réponse

L'agent A demande à l'agent B d'effectuer une tâche. L'agent A configure un point de surveillance pour détecter l'apparition du fichier de réponse de B :

```bash
# Agent A : Enregistrer un point de surveillance pour le fichier de réponse attendu
./allinker watch add --name "resp-auth-module" -d ./CodeX -p "RESP_*.md" --user TRAE

# Agent A : Bloquer jusqu'à l'apparition du fichier (timeout 300s)
./allinker wait -d ./CodeX -f "RESP_*.md" -t 300

# Agent A : Vérifier si la réponse est arrivée
./allinker watch check --name "resp-auth-module"

# Lister tous les points de surveillance actifs
./allinker watch list

# Supprimer un point de surveillance
./allinker watch remove --name "resp-auth-module"
```

---

## Mode Serveur — Collaboration LAN Multi-Hôtes

allinker peut fonctionner comme un service HTTP long terme, permettant aux agents sur **différents hôtes du même LAN** de l'appeler via le réseau. C'est le mécanisme central de la collaboration en équipe multi-machine.

```bash
# Démarrer le serveur
./allinker -server --port 8080

# Mode client (connexion à un serveur distant)
./allinker --connect http://127.0.0.1:8080 lock -f PLAN_001.md --user TRAE

# Mode automatique : utilise le réseau si un serveur est disponible, sinon exécute localement
./allinker --auto send --at CodeX --msg "bonjour" --user TRAE

# Gestion du serveur
./allinker -server --stop
./allinker -server --status
```

### API HTTP

| Point d'accès | Méthode | Description |
|---------------|---------|-------------|
| `/api/v1/command` | POST | Exécuter une commande à distance |
| `/api/v1/health` | GET | Vérification de l'état |
| `/api/v1/status` | GET | État du service |

---

## Compilation Multi-Plateforme

Sur Windows, exécutez le script de compilation inclus pour produire des binaires cross-plateforme :

```bat
build.bat
```

Génère :

| Binaire | Plateforme |
|---------|-----------|
| `allinker_windows_amd64.exe` | Windows x64 |
| `allinker_windows_386.exe` | Windows x86 |
| `allinker_linux_amd64` | Linux x64 |
| `allinker_linux_arm64` | Linux ARM64 |
| `allinker_darwin_amd64` | macOS Intel |
| `allinker_darwin_arm64` | macOS Apple Silicon |

---

## Codes de sortie

| Code | Signification |
|------|--------------|
| 0 | Succès |
| 1 | Erreur générale |
| 2 | Temps d'attente dépassé (wait) |
| 3 | Échec d'acquisition du verrou (tryLock) |
| 4 | Le compte n'existe pas |
| 5 | Permissions insuffisantes |
| 6 | Le fichier n'existe pas |

---

## Stockage des données

Toutes les données sont stockées dans le répertoire `.alf/` (configurable via `--data-dir`) :

```
.alf/
├── users.json        # Comptes utilisateur
├── config.json       # Configuration de l'outil
├── counter.json      # Compteur d'ID
├── watchlist.json    # Registre des points de surveillance
├── allinker.db       # Base de données SQLite (messages + verrous + points de surveillance)
└── Logs/             # Fichiers journaux (rotation quotidienne : YYYY-MM-DD.log)
```

Les opérations d'écriture utilisent des **écritures atomiques** (fichier temporaire → renommage) pour prévenir la corruption des données.

---

## Structure du projet

```
.
├── main.go        # Point d'entrée
├── go.mod
├── build.bat      # Script de compilation multi-plateforme
├── account/       # Gestion des comptes
├── cli/           # Routage des commandes CLI
├── config/        # Gestion de la configuration
├── core/          # Singletons globaux
├── init/          # Initialisation du répertoire de données et de la base
├── lock/          # Verrouillage de fichiers
├── logutil/       # Journalisation et audit
├── message/       # Messagerie
├── model/         # Modèles de données
├── storage/       # Persistance JSON
├── wait/          # Attente bloquante de fichiers
└── watch/         # Surveillance de fichiers
```

---

## Licence

[Apache License 2.0](../LICENCE)
