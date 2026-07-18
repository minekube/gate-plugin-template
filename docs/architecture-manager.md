# Hardcore Together Manager アーキテクチャ設計

`specification.md`（以下「仕様書」）の内容を、このリポジトリ（**Managerのみ**）にどう実装として落とし込むかの設計。仕様書はGate/Manager2プロセス構成（仕様書1節）を前提としているが、**Gate・hardcore MOD・lobby MODはいずれも別プロジェクト（別リポジトリ）として実装される**ため、本ドキュメントの対象はManager側の実装に限定する。実装前の設計合意用ドキュメントであり、コード自体はまだ書いていない（本リポジトリは現時点でドキュメントのみ）。

Gate⇔Manager間・MOD⇔Manager間プロトコルの詳細（メッセージのフィールド定義・JSON例・シーケンス図）は`docs/protocol-gate-manager.md`・`docs/protocol-mod-manager.md`が正であり、本ドキュメントでは重複させず参照するに留める。

## 0. 前提・スコープ

- Managerは**常駐プロセス**であり、hardcoreサーバーを`os/exec`の子プロセスとして起動/停止/再起動する（仕様書1節）。この親子プロセス関係のため、Manager・hardcoreサーバーは同一コンテナ上で動作する必要がある
- Managerは2本のTCP+NDJSONサーバーを持つ：hardcore MOD向け（`docs/protocol-mod-manager.md`、`127.0.0.1`限定）とGate向け（`docs/protocol-gate-manager.md`、Docker network内）。**いずれもManagerがサーバー側（listenする側）**であり、MOD・Gateがそれぞれクライアントとして接続しにくる（両プロトコルの1節参照）
- hardcore MOD・lobby MOD・Gateの実装はブラックボックスとして扱う。Managerが提供する契約は上記2つのプロトコルドキュメントのみ

## 1. リポジトリ構成

```
cmd/manager/
  main.go                                   エントリポイント。`--config`フラグ（デフォルト`./config.yml`、
                                             9節参照）でconfig読み込み→各サブシステムの起動→
                                             シグナル受信（SIGTERM等）でのgraceful shutdown

internal/
  config/
    config.go                               config.yml読み込み・バリデーション（9節）
  state/
    state.go                                 停止中/起動処理中/準備完了の3値状態＋runningキャッシュを
                                              1つのMutexで保護する状態ストア（2節）
  process/
    process.go                               os/execでhardcoreプロセスの起動/停止、標準出力/エラー出力の
                                              ログ転送、終了検知（3節）
    worldgen.go                               /start時のworld/削除＋server.propertiesのhardcore設定保証
                                              （テンプレートコピーではなく、都度新規シードで生成させる。3節）
  archive/
    archive.go                               world/ → archive/<name>/world/ コピー、meta.json書き込み、
                                              name/createdAtの生成（name省略時）・名前重複チェック・
                                              連番付与（4節）
    restore.go                               archive/<name>/world/ → world/ 復元（/load用、4節）
  records/
    records.go                               records/<challengeId>.json の直接読み取り、/savedata・/senpan
                                              の集計ロジック（5節）
  modserver/
    server.go                                MOD⇔Manager TCP+NDJSONサーバー本体（127.0.0.1:signalPort）
    handler.go                               ready/running-changed/archive-request受信、archive-complete送信
                                              （6節、docs/protocol-mod-manager.md準拠）
  gateserver/
    server.go                                Gate⇔Manager TCP+NDJSONサーバー本体（設定可能な待受アドレス）
    handler.go                               state-query/start/load/savedata-query/senpan-query受信、
                                              evacuate-request/hardcore-ready送信
                                              （7節、docs/protocol-gate-manager.md準拠）
  orchestrator/
    orchestrator.go                          state・process・archive・modserver・gateserverを束ねる中心。
                                              /start・/loadの一連シーケンス（仕様書7.3節）はここに実装する（8節）
  internal/mocktcp/
    mocktcp.go                               テスト専用：MOD側・Gate側のNDJSONクライアントを模したヘルパー
                                              （13節）
```

Gate側リポジトリの`architecture-gate.md`（別リポジトリ）冒頭に「Manager側の実装設計はManager側プロジェクトの`architecture.md`（別リポジトリ）に記載される想定」という記述があるが、本ドキュメント（`architecture-manager.md`）がそれに相当する。

## 2. 状態管理設計（`internal/state`）

Managerが内部で持つ状態は2つで、常にペアで扱う（仕様書3.1節）。

| 状態 | 型 | 意味 |
|---|---|---|
| `phase` | `stopped` \| `starting` \| `ready` | プロセスライフサイクル上の3値状態（仕様書3.1節の状態遷移図） |
| `running` | `true` \| `false` \| `unknown` | 挑戦が進行中かどうかのキャッシュ。hardcore MODからの`ready`/`running-changed`で更新 |

- 1つの`sync.RWMutex`で`{phase, running}`のタプルを保護する`Store`型を用意し、読み取りは`Snapshot() (phase, running)`、更新は専用メソッド（`MarkStarting()`, `MarkReady(running bool)`, `SetRunning(bool)`, `MarkUnknown()`）経由のみに限定する（フィールドを直接触らせない）
- **安全側デフォルト**：Manager起動直後、およびMOD⇔Manager接続が切断された場合は`running=unknown`とし、Gate側の`running`チェック（`/start`・`/load`）は`unknown`を`true`と同じ扱い（拒否）にする（仕様書3.1節・7節`state-response`の`running`値に`unknown`という第3値がある理由はこれ）
- `phase`が`starting`の間は、MOD⇔Manager接続がまだ確立していない（新プロセスがこれから`ready`を送ってくる）ため、この間の`running`は自然と`unknown`になる。**これにより「起動処理中に別の`/start`が来たらどうするか」を専用のロックで排他する必要が無い**——`running=unknown`は拒否対象なので、2件目の`/start`はrunningチェックだけで自動的に弾かれる（`force`指定時は別、8節参照）

## 3. プロセスライフサイクル管理（`internal/process`）

`os/exec.Cmd`でhardcoreサーバーを子プロセスとして起動する。

- **起動**：`exec.CommandContext`＋`config.hardcore.startCommand`（9節）。標準出力/標準エラーはManagerのログへ行単位で転送する（クラッシュ時の調査用）。`cmd.Wait()`の結果（exit code）はログに残すのみで、Manager側のビジネスロジックはMOD⇔Manager接続の断（2節）で検知する
- **停止**：まずプロセスへ`SIGTERM`（NeoForge/バニラサーバーは`stop`コマンド相当のgraceful shutdownに対応しないため、シグナルベースで統一する）を送り、`cmd.Wait()`をタイムアウト付きで待つ。タイムアウトした場合のみ`SIGKILL`にエスカレーションする
- **ワールドの新規生成（`/start`用）**：`world/`ディレクトリを削除するだけで、テンプレートのコピーは行わない。**シード値は都度やり直したい**（毎回ランダムな新しいワールドで挑戦する）ため、あらかじめ焼き込んだワールドを複製する方式は採らず、`world/`が存在しない状態でプロセスを起動し、NeoForge（バニラ準拠）自身に新規ワールドを生成させる
- **hardcoreモード・難易度HARDの固定方法**：バニラサーバーは`server.properties`の`hardcore=true`を新規ワールド生成時に読むと、そのワールドをハードコアモード（難易度HARD固定・死亡でスペクテイター送り）で生成する、という標準機能を持つ（NeoForgeもこれをそのまま継承しており、MOD側でランタイムに`setHardcore`を呼ぶ必要はない）。同様に`level-seed`を空にしておけば、新規生成のたびにランダムなシードが使われる。つまり**「テンプレートに焼き込む」必要は無く、`hardcore/`作業ディレクトリに置く`server.properties`で`hardcore=true`・`level-seed=`（空）にしておくだけで、仕様書5.3節の要件（ランタイムでの`setHardcore`相当APIが無い制約下でのhardcore固定）とユーザーが望む「シードは都度やり直す」の両方を満たせる
- **Managerによる`server.properties`の保証**：`server.properties`自体は`world/`の外にあり`/start`のワイプ対象ではない（仕様書11節）ため、通常は初期セットアップ時に設定した値がそのまま残り続ける想定だが、手動編集等で`hardcore=true`が意図せず外れる事故を防ぐため、Managerは`/start`時に`world/`を削除する前後で`server.properties`の`hardcore=true`を読み取り検証し、`false`になっていた場合は書き戻す（`level-seed`は明示的に空へ強制はしない——運用上あえて固定シードでテストしたいケースを妨げないため。デフォルトで空にしておく運用は初期セットアップ側の責務とする）
- **`records/`はワイプ対象に含めない**：`world/`と同階層だが別ディレクトリなので、`world/`削除処理は`records/`に触れない（仕様書11節の table通り）

## 4. アーカイブ実行（`internal/archive`）

`archive-request`受信時（`modserver`経由、6節）、Managerは以下を行う（仕様書3.2節）。

1. `now := time.Now()`を読む（このタイムスタンプを`createdAt`、および`name`省略時は`name`の生成にも使う——両者が同一の値になるよう、必ず1回だけ読んで使い回す）
2. 受信した`name`が空でなければそのまま使う。空（省略）なら、`now`を`2026-07-18T12-34-56`形式に整形して`name`とする
3. `archive/<name>/`の存在チェック
4. 存在すれば分岐：`archive-request`で`name`を送っていた場合（手動）は拒否（`archive-complete`を返さない。7節参照の`archive-rejected`案は未実装）／省略していた場合（自動）は末尾へ連番を付与した名前で3.に戻る
5. `world/` → `archive/<name>/world/`をコピー（hardcoreプロセスは止めない。MOD側が`save-off`→`save-all flush`済みの状態で送ってくる前提、5.2〜5.3節）
6. `archive/<name>/meta.json`に`{"elapsedTime": ..., "createdAt": now}`を書き込む（仕様書11節でファイル名を確定済み。`elapsedTime`は受信した値、`createdAt`は1.の`now`）
7. `archive-complete{name: <実際に採用したname>}`をMODへ返す（`modserver`経由）

**`name`・`createdAt`の生成元をManagerに一本化した**（`docs/protocol-mod-manager.md` 3.3節）。`name`を送った場合は拒否、省略した場合は連番付与、という分岐自体は変わらないが、この分岐は「手動/自動」を区別する専用フィールドではなく**`name`が空かどうかだけ**で判定する。理由：
- 当初、Manager側だけでは手動/自動の区別がつかない抜けがあった。`name`の命名規則〔タイムスタンプ形式か否か〕から推測する案も検討したが結合度が高く見送り、`origin`（`"manual"` | `"auto"`）フィールドを追加して解消した
- その後さらに見直し、`origin`自体を廃止した。`name`・`createdAt`の生成元をMODからManagerへ移した結果、`name`は「送るか省略するか」の任意フィールドになり、この有無自体が手動/自動を過不足なく表すようになったため、重ねて`origin`を持つのは冗長だった
- あわせて、ボス討伐時の日時整形・名前生成ロジックをMOD側に持たせる必然性も無い（実際にファイルコピーとタイムスタンプ発行を行うのはManagerであり、MOD・Managerは同一コンテナ上でクロックを共有しているため、MODが別途計測・整形して送る意味が無い）と判断し、`name`（省略時）・`createdAt`（常に）ともにManager側で生成する設計にした
- この変更により、`name`を省略した場合MODは`archive-request`送信時点で最終的な`name`を知らない。`archive-complete`の`name`で通知し、MODはそれを5.5節のイベントログ（`archiveName`）等に使う

- **`/load`用の復元（`restore.go`）**：`archive/<name>/world/` → `world/`のコピー（3節のワイプと同じく、コピー前に既存`world/`を削除する）
- **排他制御**：「アーカイブ実行中は`/start`・`/load`をブロックする」（仕様書3.2節）を、`orchestrator`が持つ1本の`sync.Mutex`（`opMutex`）で実現する。アーカイブコピーも、`/start`・`/load`のプロセス再起動シーケンス（8節）も、この同じ`opMutex`を獲得してから実行する。仕様書の文言が「ブロックする」（＝拒否ではなく待たせる）である以上、`TryLock`ではなく`Lock()`（ブロッキング）を使う——アーカイブコピーは通常数秒〜数十秒で終わる短時間処理なので、`/start`・`/load`側が多少待たされても実用上問題ない

## 5. 挑戦記録の読み取り（`internal/records`）

- `config.hardcore.recordsDir`配下の`*.json`を全件走査し、各ファイルを`{challengeId, lastKnownElapsedTime, events[]}`としてパースする（仕様書5.5節のファイル構造）
- **書き込みは行わない**（書き込みはhardcore MODの責務、仕様書3.3節）。ファイルロック等の配慮も不要（Managerは読み取り専用）
- `/savedata`：全ファイルの`events`を`challengeId`付きでフラットにマージして返す（`savedata-response`、`docs/protocol-gate-manager.md` 3.6節）
- `/senpan list|count`：全ファイルの`events`から`type: death`のみを抽出し、`deadPlayer.uuid`でグルーピングして件数・一覧を返す（`senpan-response`、同3.7節）
- **`config.hardcore.recordsDir`はhardcore MODの`storage.recordsDir`と値を一致させる必要がある**（仕様書3.3節・5.5節）。Managerはこの一致を実行時に検証できない（MOD側の設定ファイルを直接読まないため）ので、`config.yml`のコメントで明記するに留める

## 6. MOD⇔Manager サーバー（`internal/modserver`）

`docs/protocol-mod-manager.md`のサーバー側実装。`127.0.0.1:<signalPort>`でリッスンし、hardcore MODからの接続を受け付ける（1節）。

- MODは`ServerStartedEvent`発火時に接続しにくるクライアント側であり、Managerは常時リッスンしているだけでよい。1本のTCP接続を「現在のhardcoreプロセスとの接続」として保持する（同時に複数のhardcoreプロセスが動くことは無い前提、1節）
- 受信：`ready`（`state.MarkReady(running)`を呼ぶ）、`running-changed`（`state.SetRunning(running)`）、`archive-request`（`archive.Execute`を呼び、完了後`archive-complete`を送信）
- 接続が切れたら`state.MarkUnknown()`を呼ぶ（2節の安全側デフォルト、仕様書6.1節の「接続断の扱い」）
- 接続断後の新規接続を新しい「現在の接続」として扱う（`/start`・`/load`で子プロセスが再起動されるたびにMOD側は再接続してくるため）

## 7. Gate⇔Manager サーバー（`internal/gateserver`）

`docs/protocol-gate-manager.md`のサーバー側実装。設定可能なアドレス（Docker network内限定、ホストへは公開しない）でリッスンする。

| 受信 | 処理 |
|---|---|
| `state-query` | `state.Snapshot()`をそのまま`state-response`として返す（同期応答） |
| `start` | `orchestrator.Start(force, requestedBy)`を呼ぶ（8節） |
| `load` | `orchestrator.Load(name, force, requestedBy)`を呼ぶ（8節） |
| `savedata-query` | `records.SaveData()`の結果を`savedata-response`で返す |
| `senpan-query` | `records.Senpan(mode)`の結果を`senpan-response`で返す |

送信（`orchestrator`からのコールバック経由）：`start-rejected`/`load-rejected`（拒否理由付き）、`evacuate-request`→`evacuate-complete`待ち、`hardcore-ready`。

Gate側は起動時に接続しにくるクライアントであり、Managerは常時リッスンする。Gate接続が切れている間に`start`/`load`は届かないため、`orchestrator`側で「Gate接続の有無」を気にする必要は無い（Gateが状態不明として振る舞うだけ、仕様書2.1節）。

## 8. `/start`・`/load`シーケンスの実装（`internal/orchestrator`）

仕様書7.3節のフローをそのままコードへ落とし込む中心コンポーネント。

```
Start(force bool, requestedBy string) result:
  1. opMutex.Lock() → defer Unlock()
  2. force==false かつ state.running が true/unknown なら
       state-rejected相当のエラーを返す（"挑戦が進行中です"）→ ここで終了
  3. state.MarkStarting()
  4. gateserver経由で evacuate-request(reason="reset" または force時"force-reset") 送信
     → evacuate-complete受信までブロック（タイムアウト付き、14節）
  5. process.Stop()（3節） → process.WipeAndCopyTemplate()（3節）
  6. process.Start()（3節）
  7. modserver からの ready 受信を待つ（タイムアウト付き）
     受信時 state.MarkReady(running) が呼ばれる（6節のhandlerが担当）
  8. gateserver経由で hardcore-ready 送信
```

`Load(name, force, requestedBy)`もほぼ同じだが、手順2で追加のアーカイブ存在チェック（`archive/<name>/`の有無、`name=="latest"`の場合は全`meta.json`の`createdAt`を比較して最新を選ぶ）を行い、手順5で「テンプレートコピー」の代わりに「`archive/<name>/world/`からの復元」（4節）を行う。

- **タイムアウト**：手順4（`evacuate-complete`待ち）・手順7（`ready`待ち）はいずれも無期限ブロックしない。具体的な秒数は14節の未確定事項（Gate側の`architecture-gate.md`にも同種の未確定事項があり、双方で値を揃える必要がある）
- **`opMutex`は`Start`/`Load`/`archive.Execute`（4節）で共有する**唯一のロックであり、「進行中は片方をブロックする」という仕様書3.2節の要求をこれ1本で満たす

## 9. 設定ファイル設計（`config.yml`）

**読み込み元パス**：Managerは起動時、`--config`フラグで指定されたパスから`config.yml`を読む（例：`manager --config /etc/hardcore-together/config.yml`）。フラグ省略時のデフォルトは`./config.yml`、すなわち**Managerプロセスのカレントディレクトリ直下**（＝10節の`<Managerの作業ディレクトリ>/config.yml`）。`config.hardcore.workDir`・`config.archive.dir`のような相対パス設定は、この`config.yml`自体の位置ではなく、常にManagerプロセスのカレントディレクトリ基準で解決する（設定ファイル自体の置き場所と紐付けて特別扱いはしない、単純な仕様）。

Docker運用時（11節）は、Dockerfileの`WORKDIR`を`<Managerの作業ディレクトリ>`に固定し、そこへ`config.yml`をイメージへ焼き込むかVolumeでマウントするかのどちらかにする想定。コンテナ内では常に同じ絶対パスになるため、`--config`は指定せずデフォルトの`./config.yml`のままで動く。

```yaml
signalPort: 9001                       # MOD⇔Manager、127.0.0.1限定リッスン（docs/protocol-mod-manager.md）
gateListenAddr: "0.0.0.0:9000"         # Gate⇔Manager、Docker network内限定を想定（docs/protocol-gate-manager.md）

hardcore:
  workDir: "./hardcore"                # Managerがos/execで起動する子プロセスの作業ディレクトリ
  startCommand: ["java", "-jar", "server.jar", "nogui"]
                                        # world/生成時のhardcore固定・シードは<workDir>/server.propertiesの
                                        # hardcore=true・level-seed=（空）で制御する（3節、templateDirは廃止）
  recordsDir: "records"                # hardcore MOD設定のstorage.recordsDirと必ず一致させること（5節）

archive:
  dir: "./archive"                     # archive/<name>/ の保存先（4節）

timeouts:
  evacuateCompleteSeconds: 30          # 要確定（14節）
  hardcoreReadySeconds: 120            # 要確定（14節）
  processStopSeconds: 30               # SIGTERM→SIGKILLエスカレーションまでの猶予（3節）
```

`admins`（OP UUIDリスト）や`velocitySecret`のようなプレイヤー・権限関連の設定はGate側の責務であり、Managerの`config.yml`には含めない（仕様書1節：Managerはファイル操作・プロセス管理・記録読み取りに徹する）。

## 10. ディレクトリ構成

仕様書11節の構成そのままで、テンプレート用ディレクトリは持たない（3節の通り、ワールド生成はNeoForge自身に委ねるため）。

```
<Managerの作業ディレクトリ>/
├── config.yml
├── archive/
│   └── <name>/
│       ├── world/
│       └── meta.json
└── hardcore/                        … config.hardcore.workDir
    ├── world/                        … /startで削除・再生成される（新規生成時、シードは都度ランダム）
    ├── records/
    ├── server.properties             … hardcore=true・level-seed=（空）をManagerが/start時に保証する（3節）
    ├── mods/, config/ 等
```

## 11. Docker構成

| | 内容 |
|---|---|
| 公開ポート | 無し（`signalPort`は`127.0.0.1`限定、`gateListenAddr`はDocker network内限定でホストへは公開しない） |
| Volume | `archive/`・`template/`・`hardcore/`（永続化が必要。コンテナ再作成時もアーカイブ・進行中の挑戦を失わないため） |
| ベースイメージ | Manager自体（Goバイナリ）に加え、hardcoreサーバー実行に必要なJavaランタイムを同一イメージに含める必要がある（`os/exec`で直接`java`を起動するため） |
| ネットワーク | Gateからの制御プロトコル接続（`gateListenAddr`）のみ外部（同一Docker network内）に露出。hardcoreサーバー自体のMinecraftポートはGateからのみ到達可能であればよく、ホストへの公開は不要 |

## 12. 並行性・排他制御まとめ

| ロック | 保護対象 | 種類 |
|---|---|---|
| `state.Store`の`RWMutex` | `{phase, running}` | 読み取り頻度が高い（`state-query`）ため`RWMutex` |
| `orchestrator.opMutex` | プロセス再起動シーケンス（`Start`/`Load`）とアーカイブコピー（`archive.Execute`）の排他 | ブロッキング`Mutex`（8節・4節） |

`running=unknown`が「起動処理中の多重`/start`」を自然に弾く（2節）ため、`opMutex`とは別に「起動処理中フラグ」を用意する必要は無い。`force`指定時のみ`running`チェックをスキップするが、`opMutex`自体は`force`でも免除しない（仕様書2.1節「`force`の適用範囲」：アーカイブ実行中の排他制御は`force`でも免除しないことと整合）。

## 13. テスト戦略

hardcore MOD・Gate本体が別リポジトリのため、実MOD・実Gateを繋いだe2eテストはこのリポジトリ単体では組めない。`docs/protocol-mod-manager.md`・`docs/protocol-gate-manager.md`を正としたGoの統合テストで代替する（`go test ./...`で完結、Docker不要）。

- **`internal/mocktcp`**：MOD側・Gate側それぞれの視点でNDJSON接続を張り、任意のメッセージを送受信できるテスト用クライアント。`modserver`・`gateserver`をそれぞれ実際に起動し、このモッククライアントから接続させて往復を検証する
- **`internal/process`**：実際のMinecraftサーバーJarの代わりに、テスト用の簡易実行ファイル（`echo`ループやテスト用のダミーGoバイナリ）を`startCommand`に指定し、起動/`SIGTERM`停止/タイムアウト後の`SIGKILL`エスカレーションを検証する
- **`internal/archive`**：一時ディレクトリ上で実際にファイルコピー・`meta.json`書き込みを検証する。`name`を送った場合の名前重複拒否、`name`を省略した場合の自動生成・連番付与、`createdAt`が常にManager生成であることを検証する
- **`internal/orchestrator`**：`mocktcp`（Gate役）＋テスト用プロセス（`process`役）を組み合わせ、`Start`/`Load`の一連シーケンス（8節）と`opMutex`によるアーカイブとの排他を検証する

## 14. 未確定事項・要確認ポイント（Manager側、実装着手前に確定させたい）

1. **`hardcore/`作業ディレクトリの初期セットアップ手順**（3節・10節）：`server.properties`・`mods/`・`config/`は仕様書11節で「本仕様の対象外」とされている標準NeoForgeサーバー構成だが、初回に誰が用意するか（Dockerイメージへ焼き込むのか、初回起動時にManagerが雛形を生成するのか）は未確定。3節の「Managerが`hardcore=true`を保証する」処理も、この初期ファイル一式が既に存在すること前提であり、真っさらな状態からの自動セットアップまでは範囲に含めていない
2. **`evacuate-complete`待ち・`ready`待ちのタイムアウト秒数**（8節・9節）：Gate側の`architecture-gate.md`にも関連する未確定事項があり、双方のリポジトリで値を揃える必要がある
3. **Gate⇔Manager間の接続タイムアウト・リトライ回数**（`docs/protocol-gate-manager.md` 5節と共通）
4. **MOD⇔Manager間の接続リトライ回数・バックオフ設定値**（`docs/protocol-mod-manager.md` 7節と共通）
5. **`archive-request`拒否の即時通知**（`archive-rejected`案、仕様書10節・`docs/protocol-mod-manager.md` 7節と共通の未決事項）：現状MOD側は`archive-complete`のタイムアウト（目安60秒）でしか失敗を検知できない
6. **Manager障害時の再接続後の再同期手順**：Manager自体がクラッシュ→再起動した場合、`os/exec`の子プロセス（hardcore）は道連れで死ぬのか、それとも生き残ったhardcoreプロセスへ再アタッチする経路を持つか（現設計は「Managerプロセスが親であり続ける」前提で、再起動時は子も含めて仕切り直す想定。生存中の子プロセスへの再アタッチは複雑さに見合わないため非対応とする案が有力だが未確定）
7. **`docs/protocol-gate-manager.md`・`docs/protocol-mod-manager.md`の変更フロー**：3リポジトリ（Gate・Manager・hardcore MOD）間でプロトコル定義をどう同期するか

## 変更履歴

- 初版：`specification.md`・`docs/protocol-gate-manager.md`・`docs/protocol-mod-manager.md`を踏まえ、Manager側のパッケージ構成・状態管理・プロセスライフサイクル・アーカイブ実行・records読み取り・2本のTCPサーバー・orchestrator・設定ファイル・Docker構成・排他制御・テスト戦略を設計。仕様書に明記の無かった「アーカイブ名重複の手動/自動判別」「セーブテンプレートの出自」をManager側の設計判断として明文化し、未確定事項に追加した
- 改訂：ワールド生成方式を変更。事前に焼き込んだテンプレートワールドをコピーする方式（`templateDir`）を廃止し、**`/start`のたびにNeoForge自身へ新規ワールドを生成させ、シード値は都度ランダムにやり直す**方式にした。hardcoreモード・難易度HARDの固定は、テンプレートではなく`hardcore/server.properties`の`hardcore=true`（バニラ標準機能、ランタイムAPI不要）で行い、Managerは`/start`時にこの値が外れていないか保証する（3節・9節・10節）。これに伴い14節の未確定事項も「テンプレートの出自」から「`server.properties`等の初期セットアップ手順」へ差し替えた
- 追記：`config.yml`の読み込み元パスを明記。Managerは`--config`フラグ（デフォルト`./config.yml`、＝プロセスのカレントディレクトリ直下）で指定されたパスを読む。Docker運用時はコンテナの`WORKDIR`を固定することでデフォルト値のまま運用できる（9節）
