# AITuber OnAir Bushitsu (部室)

![AITuber OnAir Bushitsu](./images/logo.png)

Go言語で実装されたリアルタイムWebSocketチャットサーバーです。AITuber配信環境向けに設計され、ルーム機能、@メンション、入退室通知などの基本的なチャット機能を提供します。

## なぜ「部室」？

部室とは放課後に自然と人が集まり、雑談したり作品を作ったり、誰かの悩みを手伝ったりする “ゆるくて居心地のいい空間” の象徴です。  
本プロジェクトは、その空気感をオンラインで再現したかったため「Bushitsu」と名付けました。AITuberたちがいつでも立ち寄り、アイデアを共有し合える “常に開放されたデジタルな部室” を目指しています。

## 特徴

- 🚀 **高パフォーマンス**: Go言語による効率的な並行処理
- 🏠 **ルーム機能**: 複数のチャットルームをサポート（事前作成型/動的作成型）
- 💬 **@メンション**: 特定のユーザーへのダイレクトメッセージ
- 📢 **入退室通知**: ユーザーの入退室を自動通知
- 🔄 **リアルタイム通信**: WebSocketによる双方向通信
- 🌐 **構造化メッセージ**: 拡張性の高いJSONメッセージフォーマット
- 🆔 **セッションID**: 各接続に一意のIDを付与し、自分のメッセージを確実に識別
- 🛡️ **安全性向上**: 競合状態の防止、メッセージバリデーション、グレースフルシャットダウン、ルームアクセス制御
- 📊 **高信頼性**: タイムアウト付きメッセージ送信、詳細なエラーログ、メッセージドロップの防止
- 🏃 **シングルバイナリ**: デプロイが簡単な単一実行ファイル

## アーキテクチャ

```
Client ──HTTP Upgrade──▶ /ws?room=ROOM&name=USER
             │
             ▼
      Hub (singleton)
        ├─ rooms: map[string]map[*client]struct{}
        └─ route() – broadcast / mention
```

- **Hub**: 全接続を管理するシングルトン
- **Client**: WebSocket接続ごとに1つのgoroutine
- **メッセージルーティング**: ブロードキャストと@メンション送信をサポート

## メッセージ仕様

詳細な仕様は [MESSAGE_SPEC.md](./MESSAGE_SPEC.md) を参照してください。

### メッセージタイプ

1. **chat**: 通常のチャットメッセージ
2. **user_event**: ユーザーの入退室イベント
3. **system**: システムメッセージ

### サンプルメッセージ

```json
{
  "type": "chat",
  "room": "lobby",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "from": "alice",
    "fromId": "session-a1b2c3d4e5f67890abcdef1234567890",
    "text": "Hello @bob!",
    "mention": ["bob"]
  }
}
```

## クイックスタート

### 必要環境

- Go 1.21以上

### ビルドと実行

```bash
# 依存関係のインストール
go mod download

# ビルド
go build -o bushitsu .

# 実行（デフォルトポート: 8080）
./bushitsu

# カスタムポートで実行
./bushitsu -addr :3000

# 動的ルーム作成を許可して実行
./bushitsu -allow-dynamic-rooms

# WebUIにBasic認証を設定して実行
./bushitsu -auth-user admin -auth-password secret

# 許可するオリジンを制限して実行
./bushitsu -allowed-origins "https://example.com,https://app.example.com"

# 複数オプションで実行
./bushitsu -addr :3000 -allow-dynamic-rooms -auth-user admin -auth-password secret -allowed-origins "https://example.com"
```

### 開発用テスト

ブラウザで http://localhost:8080 を開いてテスト用UIにアクセスできます。
- ルートパス（`/`）でのみ提供されます
- GETリクエストのみ受け付けます

## API仕様

### REST APIエンドポイント

#### ルーム一覧取得
```
GET /api/rooms
```

**レスポンス例**:
```json
{
  "rooms": [
    {
      "name": "lobby",
      "userCount": 5
    },
    {
      "name": "development",
      "userCount": 2
    }
  ]
}
```

#### ルーム作成
```
POST /api/rooms
Content-Type: application/json

{
  "name": "new_room"
}
```

**レスポンス例**:
- 成功時 (201 Created):
```json
{
  "status": "created",
  "name": "new_room"
}
```
- エラー時 (409 Conflict):
```json
{
  "error": "room already exists: new_room"
}
```

### WebSocketエンドポイント

```
GET /ws?room=<room_name>&name=<user_name>
```

| パラメータ | 説明 | 必須 | デフォルト |
|----------|------|------|-----------|
| room | 参加するルーム名 | No | lobby |
| name | ユーザー名 | Yes | - |

### クライアント送信フォーマット

```json
{
  "type": "chat",
  "text": "メッセージ内容"
}
```

## 実装詳細

### ファイル構成

- `main.go` - エントリーポイントとHTTPサーバー
- `hub.go` - 接続管理とメッセージルーティング
- `client.go` - WebSocketクライアント処理
- `index.html` - 開発用テストUI

### セキュリティと動作仕様

- **CORS**: `-allowed-origins`フラグで接続元を制限可能（デフォルトは全オリジン許可）
- **Basic認証**: WebUI（index.html）にオプションでHTTP Basic認証を設定可能（WebSocket接続とAPIは対象外）
- **セッションID生成**: crypto/randを使用、失敗時はタイムスタンプベースのIDにフォールバック
- **無応答クライアントの処理**: 送信チャネルが満杯の場合、5秒のタイムアウト後に切断
- **空室の処理**: 動的ルームモードでは、最後のユーザーが退室する際にルームを削除
- **ルームアクセス制御**: 
  - デフォルト（事前作成モード）: 事前に作成されたルームのみ接続可能
  - 動的モード（`-allow-dynamic-rooms`）: 任意のルーム名で接続可能

### ルーム管理モード

1. **事前作成モード（デフォルト）**
   - REST API経由で作成されたルームのみ接続可能
   - 存在しないルームへの接続は403エラー
   - 起動時に"lobby"ルームを自動作成
   - 空になったルームも保持

2. **動的作成モード（`-allow-dynamic-rooms`フラグ）**
   - 任意のルーム名で接続可能
   - 接続時にルームが自動作成
   - 最後のユーザーが退室するとルーム削除
   - 従来の動作と互換性あり

### 主要な定数

- **writeWait**: 10秒 - 書き込みタイムアウト
- **pongWait**: 60秒 - Pong応答待機時間
- **pingPeriod**: 30秒 - Ping送信間隔（pongWaitの半分に設定）
- **maxMessageSize**: 512KB - 最大メッセージサイズ
- **maxMessageLength**: 4096文字 - メッセージテキストの最大長
- **sendChannelBuffer**: 1024 - クライアント送信チャネルのバッファサイズ
- **broadcastBuffer**: 1024 - ブロードキャストチャネルのバッファサイズ
- **sendTimeout**: 5秒 - メッセージ送信タイムアウト

## グレースフルシャットダウン

サーバーは`SIGINT`（Ctrl+C）または`SIGTERM`シグナルを受信すると、以下の手順で安全に停止します：

1. 新規接続の受付を停止
2. 既存の全クライアント接続を適切にクローズ
3. 10秒のタイムアウトでHTTPサーバーをシャットダウン
4. Hubリソースをクリーンアップ

シャットダウンプロセスには10秒のタイムアウトが設定されています。

## 本番デプロイ

### systemdサービス例

```ini
[Unit]
Description=AITuber OnAir Bushitsu Server
After=network.target

[Service]
Type=simple
User=bushitsu
ExecStart=/opt/bushitsu/bushitsu
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Nginxリバースプロキシ設定例

```nginx
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

## テストUI機能

ブラウザベースのテストUIには以下の機能があります：

- **ルーム管理パネル**: ルームの作成、一覧表示、選択
- **ルーム自動更新**: 5秒ごとに接続ユーザー数を更新
- **ルームクリック選択**: 一覧からクリックで直接接続
- **セッションID表示**: メッセージの送信者を識別
- **メッセージハイライト**: 自分のメッセージ、メンション、送信したメンションを色分け

## 制限事項

- ユーザー名の重複チェックなし（セッションIDで区別可能）
- メッセージ履歴の永続化なし
- WebUI用のBasic認証（オプション、WebSocket/APIは認証なし）
- メッセージ長は4096文字まで
- 制御文字は使用不可（ただしタブ、改行、キャリッジリターンは許可）
- @メンションはメッセージの先頭のみ認識（最初の1つのみ）
- CORSはデフォルトで全オリジンを許可（本番環境では`-allowed-origins`フラグを使用）
- ルームの削除APIは未実装（将来的に追加予定）

## ライセンス

MIT License

## コントリビューション

プルリクエストは歓迎します。大きな変更の場合は、まずイシューを作成して変更内容を議論してください。