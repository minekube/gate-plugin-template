# Hardcore Together Gate アーキテクチャ設計

`specification.md`（以下「仕様書」）の内容を、このリポジトリ（**Gateのみ**）にどう実装として落とし込むかの設計。仕様書はGate/Manager2プロセス構成（仕様書1節）を前提としているが、**Managerは別プロジェクト（別リポジトリ）として実装される**ため、本ドキュメントの対象はGate側の実装に限定する。実装前の設計合意用ドキュメントであり、コード自体はまだ書いていない。

Gate⇔Manager間プロトコルの詳細（メッセージのフィールド定義・JSON例・シーケンス図）は`docs/protocol-gate-manager.md`が正であり、本ドキュメントでは重複させず参照するに留める。MOD⇔Manager間プロトコル（`docs/protocol-mod-manager.md`）はGateが直接関与しないため、本ドキュメントの対象外。

## 0. 前提として確認したGateの既存機能

実装方針を決める前に、Gate本体（`go.minekube.com/gate v0.65.0`）が既に提供している機能を調査した。車輪の再発明を避けるため、以下は自作せずGateの機能をそのまま使う。

### 0.1 `/server`コマンドは実装不要

仕様書2.1節の`/server`（「権限保有者のみ表示」）は、Gateに**ビルトインコマンドとして既に存在する**（`pkg/edition/java/proxy/builtin_cmd_server.go`）。

- パーミッションノード`gate.command.server`で`Requires`ガードされている
- ガードの有効/無効自体は`config.yml`の`config.requireBuiltinCommandPermissions`で切り替え（デフォルト`false`＝誰でも見える）

→ 対応：Gateの`config.yml`に`requireBuiltinCommandPermissions: true`を設定し、OPに`gate.command.server`パーミッションを付与する（付与方法は0.2節）だけでよい。独自の`/server`コマンドは実装しない。

### 0.2 パーミッションは`PermissionsSetupEvent`で付与する

Gateには`proxy.PermissionsSetupEvent`という組み込みイベントがあり、Subject（≒プレイヤー）ごとに`permission.Func(permission string) permission.TriState`を設定できる（`pkg/util/permission`）。これがGateにおける「デフォルトの権限管理システム」であり、これを使う。

ただし、Gate自体は「誰がOPか」を判断する情報源を持たない（バックエンドサーバーの`ops.json`とは独立している）。そのため、**OP判定の元データはGate自身の設定ファイルにUUIDリストとして持つ**方式にする。

```
PostLoginEvent的なタイミング → PermissionsSetupEvent発火
  → Subject（Player）のUUIDがconfigのadmins一覧に含まれるか判定
  → 含まれる場合: "hardcoretogether.admin" と "gate.command.server" をTrueにするFuncをSetFunc
  → 含まれない場合: 何もしない（未設定パーミッションはUndefined→false扱い）
```

### 0.3 forwardingは`velocity`モードを使う

仕様書1節「両サーバーにProxy-Compatible-Forge（PCF）を導入し、Velocity方式のforwardingで統一する」は、Gateの`config.forwarding.mode: velocity`にそのまま対応する。`velocitySecret`（環境変数`GATE_VELOCITY_SECRET`でも可）をGate・lobby・hardcore双方のPCF設定で共有する必要がある（3節の設定例に反映）。

### 0.4 異常切断時のフォールバック（仕様書2.4節）はコード不要、config設定のみで実現できる

Gate本体のソース（`pkg/edition/java/proxy/switch.go`の`nextServerToTry`）を確認したところ、`config.failoverOnUnexpectedServerDisconnect: true`は**`config.try`リストを順に見て、現在切断されたサーバーをスキップしながら次の接続先へ自動リダイレクトする**、という汎用フェイルオーバー機構だった。

- `try: [lobby]`のみを指定（**hardcoreは含めない**）しておけば、hardcoreから予期せず切断されたプレイヤーは自動的にlobbyへ再接続される。これは仕様書2.4節の要件を満たす
- `servers:`（サーバー登録）にはhardcoreも含めてよい（`/rta`が接続先として参照するため）。`try:`（フォールバック順）から外すことで「hardcoreへの自動フォールバック」を防ぎつつ、「hardcoreからのフォールバック先」がlobbyになる、という非対称な挙動を1つの設定だけで実現できる

## 1. リポジトリ構成

```
gate.go                                    既存。plugins/hardcoretogether.Plugin を登録するだけ

plugins/hardcoretogether/       Gate側プラグイン（proxy.Plugin実装）
  plugin.go                      proxy.Plugin定義、Init内で各サブシステムを配線
  config.go                       Gate自身の設定ロード（config.ymlのhardcoreTogetherセクション）
  permissions.go                   PermissionsSetupEventの購読、admins判定（0.2節）
  cmd_rta.go                         /rta, /lobby。stateを都度managerclientへ問い合わせる
  cmd_start.go                        /start, /start force → managerclientへstart委譲
  cmd_load.go                          /load <name|latest> [force] → managerclientへload委譲
  cmd_records.go                        /savedata, /senpan → managerclientへクエリ
  evacuate.go                            EvacuateAll（evacuate-request受信で呼ばれる）、
                                          KickedFromServerEventは購読しない（0.4節の通りconfigのみで対応）
  transfer.go                             TransferLobbyToHardcore（hardcore-ready受信で呼ばれる、
                                           lobby接続中の全プレイヤーをhardcoreへ自動接続）
  util.go                                  メッセージ整形・実行者名取得の共通ヘルパー
  commands_test.go                         コマンド層の統合テスト（7節）
  managerclient/
    client.go                              Gate→Manager TCP+NDJSON接続。送受信・再接続・
                                            コールバック登録。docs/protocol-gate-manager.md準拠
    client_test.go                         プロトコル層の統合テスト（7節）
  internal/mockmanager/
    mockmanager.go                         テスト専用のスクリプト可能な疑似Manager TCPサーバー（7節）
```

`plugins/tablist`等の既存デモプラグインと同じ「Gateプラグインは`proxy.Plugin`を1つexportする」慣例を`plugins/hardcoretogether/plugin.go`で踏襲する。

## 2. Gate側設計

### 2.1 状態の問い合わせ（キャッシュは持たない）

当初はManagerからのpush型`state`シグナルをローカルにキャッシュする設計を検討したが、`/start`・`/load`はどのみちManager側の`running`チェック・アーカイブ存在チェック・排他ロックを経由する必要があり、Gate側のキャッシュがあっても問い合わせを省略できず、キャッシュのズレ（push漏れ・再接続直後の未受信）というリスクだけが残ることが分かった。そのため**Gateは状態をキャッシュせず、必要なタイミングで`managerclient.QueryState(ctx)`を呼び毎回同期的に問い合わせる**方式にした（詳細な経緯は`docs/protocol-gate-manager.md` 6節）。

- `QueryState`は`state-query`を送って`state-response`を待つだけの単純な同期呼び出し
- `managerclient`との接続が確立していない場合は、問い合わせを送らずに即座に「状態不明」を返す

### 2.2 コマンド設計

| コマンド | 実装先 | 権限チェック | 処理概要 |
|---|---|---|---|
| `/rta` | cmd_rta.go | なし | `managerclient.QueryState`で現在状態を取得し、`Ready`のときのみhardcoreへ接続。それ以外は状態に応じたメッセージ |
| `/lobby` | cmd_rta.go | なし | 常にlobbyへ接続 |
| `/start [force]` | cmd_start.go | `hardcoretogether.admin` | `managerclient`へ`start`送信 → `start-rejected`または`evacuate-request`（→退避後は`hardcore-ready`）のいずれかで結果を受ける |
| `/load <name\|latest> [force]` | cmd_load.go | `hardcoretogether.admin` | `managerclient`へ`load`送信 → 同上 |
| `/savedata` | cmd_records.go | なし | `managerclient`へ`savedata-query`送信、`savedata-response`を整形表示 |
| `/senpan list\|count` | cmd_records.go | なし | `managerclient`へ`senpan-query`送信、`senpan-response`を整形表示 |
| `/server` | (Gate組み込み) | `gate.command.server` | 0.1節の通り実装不要 |

各コマンドは`brigodier.LiteralNodeBuilder`を返す関数として組み立て、`p.Command().Register(...)`で登録する（既存の`titlecmd`と同じパターン）。`/start`・`/load`は`managerclient`のコールバック経由で非同期に結果を受け取り、コマンド実行者へ表示する（NDJSON接続は1本の常駐コネクションであり、リクエスト/レスポンスの相関はメッセージ内容や送信順で取る簡易実装で足りる規模）。`/rta`・`/savedata`・`/senpan`は`QueryState`と同様に、送信→対応する応答（`state-response`/`savedata-response`/`senpan-response`）を待つ同期的な呼び出しとして実装できる。

### 2.3 退避設計（`evacuate.go`）

- `EvacuateAll(p *proxy.Proxy, reason string)`：hardcoreサーバーに接続中の全`Player`を列挙し、`CreateConnectionRequest(lobbyServer)`で並行転送。`reason`（`reset`/`force-reset`）によってメッセージ文面を変える
- `managerclient`が`evacuate-request`を受信したらこの関数を呼び、完了後に`evacuate-complete`をManagerへ送り返す（`docs/protocol-gate-manager.md` 3.5節のハンドシェイク）
- 仕様書2.4節（異常切断時の自動lobby復帰）は0.4節の通りGateのconfig設定のみで実現するため、`KickedFromServerEvent`の自前購読は行わない

### 2.4 自動転送（`transfer.go`）

- `TransferLobbyToHardcore(p *proxy.Proxy)`：その時点でlobbyに接続している全プレイヤーをhardcoreへ接続する
- `managerclient`が`hardcore-ready`（`/start`・`/load`完了の1回限りの通知）を受信したら呼ぶ

### 2.5 `managerclient/`

Gate起動時にManagerへTCP接続し、以後は常駐する（`docs/protocol-gate-manager.md` 1節）。

- 接続失敗・切断時は数回リトライ＋バックオフ
- `evacuate-request`は`evacuate.go`へ、`hardcore-ready`は`transfer.go`へディスパッチする。`state-response`・`start-rejected`・`load-rejected`・`savedata-response`・`senpan-response`は、対応するリクエストを送った呼び出し元（2.1節・2.2節の同期/非同期呼び出し）へ返す
- 接続が確立していない間、`/start`・`/load`等のコマンドは「Managerと接続できていません」といったメッセージを即座に返す（Managerへの送信を試みない）

## 3. Managerとの関係

**Managerは別プロジェクト（別リポジトリ）として実装される。** 本リポジトリはManagerの内部実装（プロセスライフサイクル管理・ワールドのバックアップ/アーカイブ・records読み取り・MOD⇔Manager間プロトコルの受信等）を一切持たない。

Gate側が持つ必要があるのは、`docs/protocol-gate-manager.md`で定義されたTCP+NDJSONプロトコルの**クライアント実装**（`managerclient/`、2.4節）のみである。Managerがどのように状態を管理し、どうhardcoreプロセスを起動/停止し、どうアーカイブを実装しているかは、Gateにとってはプロトコルの向こう側のブラックボックスであり、本ドキュメントの対象外とする。

Manager側のリポジトリ構成・実装設計は、Manager側プロジェクトの`architecture.md`（別リポジトリ）に記載される想定。両リポジトリ間で共有すべきは`docs/protocol-gate-manager.md`（Gate⇔Manager）の内容のみであり、変更時はどちらのリポジトリで先に直すかを決めておく必要がある（6節の未確定事項）。

## 4. Gate設定ファイル設計（`config.yml`）

```yaml
config:
  bind: 0.0.0.0:25565
  onlineMode: true
  requireBuiltinCommandPermissions: true   # 追加: /serverの権限チェックを有効化(0.1節)
  failoverOnUnexpectedServerDisconnect: true  # 追加: 異常切断時の自動lobby復帰(0.4節)
  forwarding:
    mode: velocity                          # 追加: PCFと合わせるforwarding方式(0.3節)
    velocitySecret: ${GATE_VELOCITY_SECRET}  # lobby/hardcore双方のPCF設定と共有する秘密鍵
  servers:
    lobby: lobby:25566
    hardcore: hardcore:25567                # Manager側で起動されるhardcoreサーバーのアドレス
  try:
    - lobby                                 # hardcoreを含めないのが重要(0.4節)

hardcoreTogether:
  admins:                                   # OP UUIDリスト（0.2節）
    - "11111111-2222-3333-4444-555555555555"
  managerAddr: manager:9000                  # Gate→Manager接続先（Manager側の待受アドレス）
  lobbyServer: lobby                          # config.servers内のキー名と一致させる
  hardcoreServer: hardcore                     # config.servers内のキー名と一致させる
```

Gateは`archiveDir`・`recordsDir`のようなファイルパス設定を一切持たない（Gateはファイルシステムにアクセスしないため、仕様書1節・11節）。`hardcoreServer`のアドレス（`hardcore:25567`等）は、Manager側プロジェクトがhardcoreサーバーをどう公開するかに依存するため、Manager側の設定と付き合わせて決める必要がある。

## 5. Docker構成（Gate単体）

本リポジトリの`Dockerfile`はGateのコンテナイメージのみをビルドする。Manager（+hardcoreサーバー）のイメージはManager側リポジトリで管理される。

| | 内容 |
|---|---|
| 公開ポート | `25565`（クライアント接続用） |
| Volume | 無し（Gateはファイルシステムにアクセスしないため） |
| ネットワーク | lobby・hardcoreサーバーへのMinecraftプロトコル接続、Managerへの制御プロトコル接続（`hardcoreTogether.managerAddr`）。いずれも同一Docker network内での名前解決を想定 |

Gate・lobby・Manager（+hardcore）を1つの`docker-compose.yml`で束ねるかどうかは、デプロイ構成（インフラ側リポジトリの要否）の話であり、本ドキュメントの対象外とする。

## 6. 未確定事項・要確認ポイント（Gate側、実装着手前に確定させたい）

1. **Gate⇔Manager間の接続タイムアウト・リトライ回数**（`managerclient`の実装設定値。`docs/protocol-gate-manager.md`参照）
2. **`/savedata`の表示形式**：challengeIdごとに区切って表示するか、全challengeIdのイベントを時系列マージして1本のログとして見せるか（`savedata-response`をGate側でどう整形するかの話）
3. **Manager障害時、Gate側の応答待ちをどう扱うか**：`managerclient`との接続が切れて復帰した際、進行中の`/start`等のコマンド応答待ちをタイムアウトさせるか、再接続後も待ち続けるか
4. **`docs/protocol-gate-manager.md`の変更フロー**：Gate・Manager別リポジトリ間でプロトコル定義をどう同期するか（どちらのリポジトリを正とするか、バージョニングするか等）
5. 仕様書10節記載の既存未決事項（PCFバージョン、権限ノード名の最終決定等）はGate側の実装には直接影響しないため本ドキュメントでは追跡のみ
6. **`/rta`・`/lobby`・退避/自動転送のテスト方法**：`*proxy.Proxy`の実インスタンス（プレイヤー無し、サーバー登録のみ）を作ってテストできるか要調査（7.3節）。できなければ、実Minecraftクライアント（または簡易プロトコルクライアント）を用意する本格的なe2eテストが必要になる

## 7. テスト戦略

Manager本体が別リポジトリのため、Docker上に実Managerを立てたe2eテストはこのリポジトリ単体では組めない。代わりに、`docs/protocol-gate-manager.md`を正としたGoの統合テストで代替する（Docker不要、`go test ./...`で完結）。

### 7.1 `internal/mockmanager`

Gate向けのシグナルを受け付ける疑似Manager TCPサーバー。`managerclient`とは型を共有せず、独自に`docs/protocol-gate-manager.md`のワイヤーフォーマットを再定義している——`managerclient`側の型と直接比較させることで「Goの構造体同士がたまたま噛み合っている」以上の検証（実際のJSON文字列レベルでの互換性）にするため。

- `Start(t, handler)`：`127.0.0.1:0`でリッスンし、受信した`Message`ごとに`handler`が返す0件以上の応答を書き込む
- `Push(msg)`：受信を待たずに一方的に送信（`evacuate-request`・`hardcore-ready`のような非同期pushの再現用）
- `CloseConn()`：接続を強制終了（Manager再起動・ネットワーク断のシミュレーション、再接続テスト用）
- `Received()`：受信済みメッセージのスナップショット（Gateが送った内容のアサーションに使う）

### 7.2 `managerclient/client_test.go`（プロトコル層）

`Client`を`mockmanager`に接続し、プロトコルの往復を直接検証する：`QueryState`／`Start`（拒否・受理〈`evacuate-request`経由〉）／`Load`／`SaveData`／`Senpan`の一連の呼び出し、`evacuate-request`受信時に`evacuate-complete`が自動送信されること、`hardcore-ready`のpushディスパッチ、接続断からの再接続。`docs/protocol-gate-manager.md`の3.6節・3.7節に載っているJSON例をそのままテストのフィクスチャに使っている。

**教訓（再接続テストのハング）**：`TestReconnectAfterDisconnect`は当初`context.Background()`（タイムアウト無し）でリトライする実装にしており、`go test ./...`実行時にごく低い確率で無限ハングした（`-count=50`では毎回数回に1回再現）。原因は`Connected()`が「切断検知前の古い接続」に対して一瞬`true`のままになる区間があり、そこで送ったクエリが応答不能な死んだ接続に乗ってしまうこと。`managerclient`本体や`plugins/hardcoretogether/cmd_*.go`（元々すべて`context.WithTimeout`使用）にはバグは無く、テストコード側の問題だった。**再接続系のテストで待ち受ける呼び出しには、必ずループ1回ごとに短いタイムアウト付きcontextを与えてリトライすること**（`context.Background()`をリトライループの中で直接使わない）。

### 7.3 `commands_test.go`（コマンド層）

`command.Manager.Do(ctx, src, "start")`のような低レベルAPIで、実際のMinecraftクライアント接続無しにbrigodierコマンドを実行し、`mockmanager`が受け取ったメッセージ・`command.Source`（自作の`fakeSource`）が受け取った応答メッセージの両方を検証する。対象は`/start`・`/load`・`/savedata`・`/senpan`（いずれも`d.proxy`に触れないコマンド）。パーミッションチェック（`hardcoretogether.admin`）が実際に弾くことも検証する。

**対象外（このテスト層では検証しない）**：`/rta`・`/lobby`、および`evacuate.go`・`transfer.go`（`onEvacuateRequest`・`onHardcoreReady`）は、実プレイヤー接続または`*proxy.Proxy`の実インスタンスを必要とするため対象外とした。将来これらを検証したくなった場合は、`proxy.New`で実サーバーを起動せずに`RegisteredServer`だけ登録したテスト用Proxyインスタンスを作れないか、別途調査が必要（未確定事項に追加）。

## 変更履歴

- 初版：`specification.md`全節（Gate単独プロセス版）を踏まえたパッケージ構成・状態管理・コマンド・プロセス管理・アーカイブ・records・シグナル通信・設定ファイルの設計を作成。Gate本体調査により`/server`実装不要・パーミッションは`PermissionsSetupEvent`を使う方針を確定
- 追記：Gate公式config.ymlテンプレートを精査し、`forwarding.mode: velocity`、`failoverOnUnexpectedServerDisconnect`＋`try`リストを採用
- 全面改訂：`specification.md`のGate/Manager分離に合わせて、Gate・Manager双方の実装をこのリポジトリで扱う構成に書き直した
- 再改訂：**Managerを別プロジェクト（別リポジトリ）とする方針**を受け、本リポジトリの対象をGate単体に限定。Manager側の実装設計（`manager/`パッケージ構成、Manager用`config.yml`、プロセス管理・アーカイブの実装詳細等）を削除し、Gate⇔Manager間はすべて`docs/protocol-gate-manager.md`のプロトコル越しのブラックボックスとして扱う設計に変更した
- 再改訂：push型`state`シグナルのローカルキャッシュ（`state_cache.go`）を廃止し、Gateが必要な時にManagerへ同期的に問い合わせる`QueryState`方式へ変更（2.1節）。`/start`・`/load`完了時の自動転送だけは`hardcore-ready`という1回限りの通知として`transfer.go`に切り出した（2.4節）。理由の詳細は`docs/protocol-gate-manager.md` 6節・`specification.md` 9節決定ログ参照
- 実装：`plugins/hardcoretogether/`一式（`plugin.go`, `config.go`, `permissions.go`, `util.go`, `cmd_*.go`, `evacuate.go`, `transfer.go`, `managerclient/client.go`）を実装し、`gate.go`・`config.yml`に反映。Gate v0.68.26では`Player.ID()`が`go.minekube.com/gate/pkg/util/uuid`型を返すため、`github.com/google/uuid`ではなくそちらを使用
- テスト追加：`internal/mockmanager`（疑似Manager TCPサーバー）を使ったGo統合テスト（7節）。Docker上に実Managerを立てるe2eテストは別リポジトリのManager実装が無いため今回は見送り、プロトコルドキュメント（`docs/protocol-gate-manager.md`）を正としたモックベースのテストで代替した
