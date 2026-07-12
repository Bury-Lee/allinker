# allinker — 에이전트 간 협업 게이트웨이

> 서로 다른 AI Agent 소프트웨어에 통합된 협업 진입점을 제공하여 에이전트 간 협업을 가능하게 합니다.

![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-green)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

[English](../README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [Français](README.fr.md)

---

## 개요

allinker는 동일한 프로젝트 디렉토리에서 작업하는 여러 AI Agent 도구(Cline, CodeX, 사용자 정의 에이전트 등)를 위해 설계된 **CLI 기반 협업 게이트웨이**입니다.

여러 에이전트가 동일한 프로젝트에서 독립적으로 작업할 때 흔히 발생하는 문제:

- **파일 충돌** — 여러 에이전트가 동시에 동일한 파일을 편집
- **정보 격리** — 에이전트 간 직접 통신 불가
- **추적 불가능한 작업** — 누가 언제 무엇을 했는지 감사할 수 없음

allinker는 **4가지 협업 프리미티브**로 이러한 문제를 해결합니다:

| 프리미티브 | 해결하는 문제 |
|-----------|--------------|
| **파일 잠금** | 편집 전에 잠금을 획득하여 충돌 방지 |
| **메시징** | 에이전트 간 `@` 멘션 메시지 전송 |
| **파일 감시** | 감시 지점을 등록하여 동료 진행 상황 파악 |
| **계정 관리** | 신원 확인 + 3단계 권한 + 전체 감사 추적 |

---

## 빠른 시작

### 빌드

```bash
git clone <repo-url>
cd allinker
go build -o allinker.exe .
```

Windows(x64/x86), Linux(x64/ARM64), macOS(Intel/ARM)용 사전 빌드 바이너리도 제공됩니다.

### 에이전트 등록

```bash
./allinker register --name TRAE --role agent
./allinker register --name CodeX --role agent
./allinker register --name admin --role admin
```

### 파일 잠금

```bash
./allinker lock -f PLAN_001.md -t 30 --user TRAE    # 블로킹 잠금 (최대 30초)
./allinker tryLock -f PLAN_001.md --user TRAE        # 비블로킹 시도
./allinker unlock -f PLAN_001.md --user TRAE         # 잠금 해제
./allinker status -f PLAN_001.md                     # 잠금 상태 확인
./allinker status --all                              # 전체 잠금 목록
```

### 메시징

```bash
./allinker send --at CodeX --msg "인증 모듈을 구현해 주세요" --user TRAE
./allinker send --at All --msg "전체 공지" --user TRAE
./allinker recv                                                   # 메시지 수신
./allinker history --with CodeX --limit 10                        # 대화 기록
```

### 파일 감시 — 응답 파일 대기

Agent A가 Agent B에게 작업을 요청하고, B의 응답 파일을 감시합니다:

```bash
# Agent A: 예상 응답 파일에 대한 감시 지점 등록
./allinker watch add --name "resp-auth-module" -d ./CodeX -p "RESP_*.md" --user TRAE

# Agent A: 파일이 나타날 때까지 블로킹 (300초 타임아웃)
./allinker wait -d ./CodeX -f "RESP_*.md" -t 300

# Agent A: 응답 도착 확인
./allinker watch check --name "resp-auth-module"

# 전체 감시 지점 목록
./allinker watch list

# 감시 지점 제거
./allinker watch remove --name "resp-auth-module"
```

---

## 서버 모드 — 호스트 간 LAN 협업

allinker는 상주 HTTP 서비스로 실행되어 **동일 LAN의 다른 호스트**에 있는 에이전트가 네트워크를 통해 호출할 수 있습니다. 이것이 다중 머신 팀 협업의 핵심 메커니즘입니다.

```bash
# 서버 시작
./allinker -server --port 8080

# 클라이언트 모드 (원격 서버 연결)
./allinker --connect http://127.0.0.1:8080 lock -f PLAN_001.md --user TRAE

# 자동 모드: 서버가 있으면 네트워크 사용, 없으면 로컬 실행
./allinker --auto send --at CodeX --msg "안녕하세요" --user TRAE

# 서버 관리
./allinker -server --stop
./allinker -server --status
```

### HTTP API

| 엔드포인트 | 메서드 | 설명 |
|-----------|-------|------|
| `/api/v1/command` | POST | 명령 원격 실행 |
| `/api/v1/health` | GET | 상태 확인 |
| `/api/v1/status` | GET | 서비스 상태 |

---

## 전체 플랫폼 빌드

Windows에서 포함된 빌드 스크립트를 실행하면 크로스 플랫폼 바이너리가 생성됩니다:

```bat
build.bat
```

생성물:

| 바이너리 | 플랫폼 |
|---------|-------|
| `allinker_windows_amd64.exe` | Windows x64 |
| `allinker_windows_386.exe` | Windows x86 |
| `allinker_linux_amd64` | Linux x64 |
| `allinker_linux_arm64` | Linux ARM64 |
| `allinker_darwin_amd64` | macOS Intel |
| `allinker_darwin_arm64` | macOS Apple Silicon |

---

## 종료 코드

| 코드 | 의미 |
|------|------|
| 0 | 성공 |
| 1 | 일반 오류 |
| 2 | 타임아웃 (wait) |
| 3 | 잠금 획득 실패 (tryLock) |
| 4 | 계정이 존재하지 않음 |
| 5 | 권한 부족 |
| 6 | 파일이 존재하지 않음 |

---

## 데이터 저장

모든 데이터는 `.alf/` 디렉토리에 저장됩니다 (`--data-dir`로 변경 가능):

```
.alf/
├── users.json        # 사용자 계정
├── config.json       # 도구 설정
├── counter.json      # ID 카운터
├── watchlist.json    # 감시 지점 등록
├── allinker.db       # SQLite 데이터베이스 (메시지+잠금+감시 지점)
└── Logs/             # 로그 파일 (일별 로테이션: YYYY-MM-DD.log)
```

쓰기 작업은 **원자적 쓰기**(임시 파일 → 이름 변경)를 사용하여 데이터 손상을 방지합니다.

---

## 프로젝트 구조

```
.
├── main.go        # 진입점
├── go.mod
├── build.bat      # 크로스 플랫폼 빌드 스크립트
├── account/       # 계정 관리
├── cli/           # CLI 명령 라우팅
├── config/        # 설정 관리
├── core/          # 전역 싱글톤
├── init/          # 데이터 디렉토리 및 데이터베이스 초기화
├── lock/          # 파일 잠금
├── logutil/       # 로깅 및 감사
├── message/       # 메시징
├── model/         # 데이터 모델
├── storage/       # JSON 영속화
├── wait/          # 블로킹 파일 대기
└── watch/         # 파일 감시
```

---

## 라이선스

[Apache License 2.0](../LICENCE)
