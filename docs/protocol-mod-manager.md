# MOD ⇔ Manager シグナルプロトコル

`specification.md` 6節の詳細版。hardcore MOD（NeoForge、Kotlin/Java）とManager（Go）の間で、hardcoreサーバーの起動完了・状態変化・アーカイブ要求をやり取りするためのプロトコル。

## 1. 前提・トランスポート

| 項目 | 内容 |
|---|---|
| トランスポート | TCPソケット |
| 待受アドレス | `127.0.0.1:<signalPort>`（Manager側、Managerの設定ファイルで`signalPort`を指定） |
| 接続方向 | hardcore MOD → Manager（MODがクライアント） |
| 接続タイミング | MODは`ServerStartedEvent`発火時に接続を開始し、成立後直ちに`ready`を送信する |
| 再接続 | 接続失敗時は数回リトライ＋バックオフして諦める（ログ出力のみ、致命的エラーにはしない） |
| メッセージ形式 | NDJSON（Newline-Delimited JSON）。1メッセージ＝1行のUTF-8 JSONオブジェクト＋`\n` |
| 判別方法 | 各メッセージの`type`フィールドで種別を判別する |
| セキュリティ | `127.0.0.1`限定リッスンにより、同一コンテナ内通信であることを前提にTLS/認証は行わない |

Managerとhardcoreサーバーは`os/exec`の親子プロセスとして**同一コンテナ内**で動作するため、`127.0.0.1`はコンテナ内ループバックとして解決される。MOD側はこの接続先アドレスを設定ファイルで持つ（Manager側の`signalPort`と値を一致させる必要がある）。

## 2. メッセージ一覧

| `type` | 方向 | 発生タイミング |
|---|---|---|
| `ready` | MOD → Manager | `ServerStartedEvent`発火時（起動完了直後、1回のみ） |
| `running-changed` | MOD → Manager | `running`の値が変化するたび（`/start`によるフレッシュ生成時の`true`初期化、全滅/挑戦終了系ボス討伐による`false`化） |
| `archive-request` | MOD → Manager | `/archive <name>`実行時（`name`あり）、または指定ボス討伐等による自動アーカイブ時（`name`省略）（`save-off`→`save-all flush`実行済みの状態で送信） |
| `archive-complete` | Manager → MOD | Managerがワールドフォルダのコピーを完了した時 |

## 3. メッセージ詳細

### 3.1 `ready`

MOD → Manager。起動完了を通知し、Managerが保持する`running`キャッシュの初期値を渡す。

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `type` | string | ✓ | 固定値 `"ready"` |
| `running` | bool | ✓ | 起動直後の`running`値（`SavedData`から読み込んだ値、またはフレッシュ生成時は`true`） |

```json
{"type":"ready","running":true}
```

### 3.2 `running-changed`

MOD → Manager。`running`フラグが変化するたびに送信する。

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `type` | string | ✓ | 固定値 `"running-changed"` |
| `running` | bool | ✓ | 変化後の`running`値 |

```json
{"type":"running-changed","running":false}
```

### 3.3 `archive-request`

MOD → Manager。`save-off`→`save-all flush`実行後に送信し、Managerによるワールドコピーを要求する。

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `type` | string | ✓ | 固定値 `"archive-request"` |
| `name` | string | 任意 | アーカイブ名。OPが指定した値。**省略した場合はManagerが自動生成する**（後述） |
| `elapsedTime` | int64 | ✓ | 経過時間（秒数、long） |

```json
{"type":"archive-request","elapsedTime":600}
```
```json
{"type":"archive-request","name":"save1","elapsedTime":600}
```

`createdAt`は含めない：作成日時はMODの送信内容に依存せず、**Manager自身が`archive-request`処理時点の現在時刻から生成する**（`meta.json`へ書き込む値、`specification.md` 3.2節）。MOD・Managerは同一コンテナ上で動作し（1節）クロックが共有されるため、MOD側で改めて計測・送信する意味が無い。

**`name`の有無による生成元と名前重複時の挙動の分岐**：（`specification.md` 3.2節。手動/自動を区別する専用フィールドは持たず、`name`が送られているかどうかだけで一意に決まる）
- **`name`を送った場合**（手動`/archive <name>`）：MODが送った`name`をそのまま使う。`archive/<name>/`が既に存在する場合は拒否する（上書きしない）。MODはこれをOPへ「その名前は既に使われています」と表示する
- **`name`を省略した場合**（ボス討伐等による自動アーカイブ）：**Managerが`archive-request`処理時点の現在時刻から`name`を生成する**（`createdAt`と同じタイムスタンプ、`2026-07-18T12-34-56`形式）。同一秒内に複数のボスが討伐される稀なケースに備え、衝突時はManager側で末尾に連番を付与して回避する（`2026-07-18T12-00-00-2`等）。失敗させずに継続させる

`name`を省略した場合、MODは`archive-request`送信時点で最終的な`name`を知らない。Managerが実際に採用した名前（連番付与後を含む）は`archive-complete`の`name`で通知するので、MODはそれを使う（3.4節）。

`deadPlayerUUID`は含めない（死亡記録は挑戦記録データ側〔`specification.md` 5.5節〕へ完全移行済みのため不要）。

### 3.4 `archive-complete`

Manager → MOD。ファイルコピー完了を通知する。MODはこれを受けて`save-on`を実行する。

| フィールド | 型 | 必須 | 説明 |
|---|---|---|---|
| `type` | string | ✓ | 固定値 `"archive-complete"` |
| `name` | string | ✓ | **Managerが実際に採用した最終的なアーカイブ名**（連番付与済みの場合はそれを含む）。`archive-request`で`name`を送っていた場合は通常それと一致する。省略していた場合、MODはこの値で初めて名前を知る |

```json
{"type":"archive-complete","name":"2026-07-18T12-00-00"}
```

## 4. 同期待ちの規約

MODは`archive-request`送信後、**次に届く`archive-complete`**を受信するまで`save-on`を実行せずに待つ。これにより、コピー中にオートセーブが再開してしまう事態を防ぐ。1つのTCP接続上で`archive-request`は常に1件ずつ・同期的に処理される（MOD側が`archive-complete`を待ってから次の操作へ進む設計のため）ので、`name`を送っていない場合でも、次に届く`archive-complete`が対応する応答であることに変わりはない。

名前重複によりManagerが`archive-request`を拒否した場合、現状**明示的な拒否シグナルは存在しない**（`specification.md` 10節の未決事項）。MODは`archive-complete`が一定時間（目安60秒、要確定）来ないことをもって失敗と判断し、OPへエラー表示する。

## 5. 接続断の扱い

TCP接続が切れた場合、Managerはhardcoreの状態を「不明」とみなし、`running`キャッシュを安全側（`true`扱い）に倒す。これにより`/start`・`/load`が誤って進行中の挑戦を破棄することを防ぐ（`specification.md` 3.1節）。

## 6. シーケンス例

```mermaid
sequenceDiagram
    participant MOD as hardcore MOD
    participant MGR as Manager

    Note over MOD: ServerStartedEvent
    MOD->>MGR: TCP接続
    MOD->>MGR: {"type":"ready","running":true}

    Note over MOD: ボス討伐（チェックポイント系）
    MOD->>MOD: save-off
    MOD->>MOD: save-all flush
    MOD->>MGR: {"type":"archive-request","elapsedTime":600}
    MGR->>MGR: 現在時刻からnameを生成、world/ を archive/<name>/ へコピー
    MGR->>MOD: {"type":"archive-complete","name":"..."}
    MOD->>MOD: save-on（受け取ったnameを以後のイベントログ記録等に使う）

    Note over MOD: 全滅
    MOD->>MGR: {"type":"running-changed","running":false}
```

## 7. 未決事項

- 接続リトライ回数・バックオフ設定値（`specification.md` 10節）
- `archive-request`拒否時の即時通知シグナル（`archive-rejected`案、未実装。`specification.md` 10節）
