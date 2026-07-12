# allinker — クロスエージェント協調ゲートウェイ

> 異なる AI Agent ソフトウェアに統一された協調エントリポイントを提供し、エージェント間の協調作業を実現します。

![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-green)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

[English](../README.md) · [简体中文](README.zh-CN.md) · [한국어](README.ko.md) · [Français](README.fr.md)

---

## 概要

allinker は、同一プロジェクトディレクトリで動作する複数の AI Agent ツール（Cline、CodeX、カスタムエージェントなど）のために設計された **CLI ベースの協調ゲートウェイ** です。

複数のエージェントが同じプロジェクトで独立して作業する際、よく直面する問題：

- **ファイル競合** — 複数のエージェントが同時に同じファイルを編集
- **情報サイロ** — エージェント間の直接通信が不可
- **操作の追跡不能** — 誰がいつ何をしたかを監査できない

allinker は **4 つの協調プリミティブ** で这些问题を解決します：

| プリミティブ | 解決する課題 |
|-------------|-------------|
| **ファイルロック** | 編集前にロックを取得し、競合を防止 |
| **メッセージング** | エージェント間で `@` メンション付きメッセージを送信 |
| **ファイル監視** | 監視ポイントを登録し、他のエージェントの進捗を把握 |
| **アカウント管理** | 本人確認 + 3段階権限 + 完全監査証跡 |

---

## クイックスタート

### ビルド

```bash
git clone <repo-url>
cd allinker
go build -o allinker.exe .
```

Windows（x64/x86）、Linux（x64/ARM64）、macOS（Intel/ARM）向けのプリビルドバイナリも利用可能です。

### エージェントの登録

```bash
./allinker register --name TRAE --role agent
./allinker register --name CodeX --role agent
./allinker register --name admin --role admin
```

### ファイルロック

```bash
./allinker lock -f PLAN_001.md -t 30 --user TRAE    # ブロッキングロック（最大30秒）
./allinker tryLock -f PLAN_001.md --user TRAE        # 非ブロッキング試行
./allinker unlock -f PLAN_001.md --user TRAE         # ロック解放
./allinker status -f PLAN_001.md                     # ロック状態確認
./allinker status --all                              # 全ロック表示
```

### メッセージング

```bash
./allinker send --at CodeX --msg "認証モジュールを実装してください" --user TRAE
./allinker send --at All --msg "全体通知" --user TRAE
./allinker recv                                                   # メッセージ受信
./allinker history --with CodeX --limit 10                        # 履歴表示
```

### ファイル監視 — 応答ファイル待ち

Agent A が Agent B にタスクを依頼し、B の応答ファイルを監視します：

```bash
# Agent A: 応答ファイルの監視ポイントを登録
./allinker watch add --name "resp-auth-module" -d ./CodeX -p "RESP_*.md" --user TRAE

# Agent A: ファイルが現れるまでブロック（300秒でタイムアウト）
./allinker wait -d ./CodeX -f "RESP_*.md" -t 300

# Agent A: 応答が到着したか確認
./allinker watch check --name "resp-auth-module"

# 全監視ポイントの一覧
./allinker watch list

# 監視ポイント削除
./allinker watch remove --name "resp-auth-module"
```

---

## サーバモード — クロスホスト LAN 協調

allinker は常駐 HTTP サービスとして実行でき、**同一 LAN 上の異なるホスト** 上のエージェントがネットワーク経由で呼び出せるようになります。これはマルチマシンチーム協調の中核メカニズムです。

```bash
# サーバ起動
./allinker -server --port 8080

# クライアントモード（リモートサーバに接続）
./allinker --connect http://127.0.0.1:8080 lock -f PLAN_001.md --user TRAE

# 自動モード：サーバがあればネットワーク経由、なければローカル実行
./allinker --auto send --at CodeX --msg "こんにちは" --user TRAE

# サーバ管理
./allinker -server --stop
./allinker -server --status
```

### HTTP API

| エンドポイント | メソッド | 説明 |
|---------------|---------|------|
| `/api/v1/command` | POST | コマンドをリモート実行 |
| `/api/v1/health` | GET | ヘルスチェック |
| `/api/v1/status` | GET | サービス状態取得 |

---

## 全プラットフォームビルド

Windows で付属のビルドスクリプトを実行すると、クロスプラットフォームバイナリが生成されます：

```bat
build.bat
```

生成物：

| バイナリ | プラットフォーム |
|----------|----------------|
| `allinker_windows_amd64.exe` | Windows x64 |
| `allinker_windows_386.exe` | Windows x86 |
| `allinker_linux_amd64` | Linux x64 |
| `allinker_linux_arm64` | Linux ARM64 |
| `allinker_darwin_amd64` | macOS Intel |
| `allinker_darwin_arm64` | macOS Apple Silicon |

---

## 終了コード

| コード | 意味 |
|--------|------|
| 0 | 成功 |
| 1 | 一般エラー |
| 2 | タイムアウト（wait） |
| 3 | ロック取得失敗（tryLock） |
| 4 | アカウントが存在しない |
| 5 | 権限不足 |
| 6 | ファイルが存在しない |

---

## データ保存

すべてのデータは `.alf/` ディレクトリに保存されます（`--data-dir` で変更可能）：

```
.alf/
├── users.json        # ユーザアカウント
├── config.json       # ツール設定
├── counter.json      # ID カウンタ
├── watchlist.json    # 監視ポイント登録
├── allinker.db       # SQLite データベース（メッセージ+ロック+監視ポイント）
└── Logs/             # ログファイル（日次ローテーション: YYYY-MM-DD.log）
```

書き込み操作は **アトミック書き込み**（一時ファイル → リネーム）を使用し、データ破損を防止します。

---

## プロジェクト構造

```
.
├── main.go        # エントリポイント
├── go.mod
├── build.bat      # クロスプラットフォームビルドスクリプト
├── account/       # アカウント管理
├── cli/           # CLI コマンドルーティング
├── config/        # 設定管理
├── core/          # グローバルシングルトン
├── init/          # データディレクトリ & データベース初期化
├── lock/          # ファイルロック
├── logutil/       # ロギング & 監査
├── message/       # メッセージング
├── model/         # データモデル
├── storage/       # JSON 永続化
├── wait/          # ブロッキングファイル待機
└── watch/         # ファイル監視
```

---

## ライセンス

[Apache License 2.0](../LICENCE)
