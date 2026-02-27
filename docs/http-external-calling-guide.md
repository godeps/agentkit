# HTTP 外部调用指南（Java / Python）

本文面向不直接嵌入 Go SDK 的调用方。  
方式是：先启动 `examples/03-http` 提供 HTTP 服务，再由 Java/Python 调用。

## 1. 启动服务端

```bash
export ANTHROPIC_API_KEY=sk-ant-...
# 或者使用 ANTHROPIC_AUTH_TOKEN（优先级更高）

go run ./examples/03-http
```

默认监听 `:8080`，可通过以下环境变量覆盖：

- `AGENTSDK_HTTP_ADDR`：监听地址（默认 `:8080`）
- `AGENTSDK_MODEL`：模型名（默认 `claude-3-5-sonnet-20241022`）

## 2. 接口概览

- `GET /health`：健康检查
- `POST /v1/run`：同步调用，返回完整 JSON
- `POST /v1/run/stream`：SSE 流式返回

请求体：

```json
{
  "prompt": "请总结项目",
  "session_id": "user-123",
  "timeout_ms": 600000
}
```

字段说明：

- `prompt`：必填
- `session_id`：可选，不传会自动生成。建议你自己传，便于会话追踪
- `timeout_ms`：可选，请求超时（毫秒）

## 3. cURL 快速验证

```bash
curl -sS -X POST http://localhost:8080/v1/run \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"hello","session_id":"demo-1"}'
```

流式：

```bash
curl --no-buffer -N -X POST http://localhost:8080/v1/run/stream \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"分析仓库结构","session_id":"demo-stream"}'
```

## 4. Python 调用示例

### 4.1 同步调用（requests）

```python
import requests

url = "http://localhost:8080/v1/run"
payload = {
    "prompt": "请输出 3 条发布说明",
    "session_id": "py-user-001",
    "timeout_ms": 300000
}

resp = requests.post(url, json=payload, timeout=310)
resp.raise_for_status()
data = resp.json()
print(data["output"])
```

### 4.2 流式调用（SSE）

```python
import json
import requests

url = "http://localhost:8080/v1/run/stream"
payload = {
    "prompt": "逐步分析这个仓库",
    "session_id": "py-stream-001",
    "timeout_ms": 600000
}

with requests.post(url, json=payload, stream=True, timeout=610) as r:
    r.raise_for_status()
    for raw in r.iter_lines(decode_unicode=True):
        if not raw:
            continue
        if not raw.startswith("data: "):
            continue
        text = raw[len("data: "):]
        evt = json.loads(text)
        t = evt.get("type")
        if t == "content_block_delta":
            delta = evt.get("delta") or {}
            print(delta.get("text", ""), end="", flush=True)
        elif t == "error":
            print("\n[ERROR]", evt.get("output", ""))
```

## 5. Java 调用示例

下面示例基于 Java 11+ `HttpClient`。

### 5.1 同步调用

```java
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public class RunExample {
  public static void main(String[] args) throws Exception {
    String body = """
      {
        "prompt":"请生成变更摘要",
        "session_id":"java-user-001",
        "timeout_ms":300000
      }
      """;

    HttpClient client = HttpClient.newBuilder()
      .connectTimeout(Duration.ofSeconds(10))
      .build();

    HttpRequest req = HttpRequest.newBuilder()
      .uri(URI.create("http://localhost:8080/v1/run"))
      .timeout(Duration.ofSeconds(310))
      .header("Content-Type", "application/json")
      .POST(HttpRequest.BodyPublishers.ofString(body))
      .build();

    HttpResponse<String> resp = client.send(req, HttpResponse.BodyHandlers.ofString());
    if (resp.statusCode() >= 400) {
      throw new RuntimeException("HTTP " + resp.statusCode() + ": " + resp.body());
    }
    System.out.println(resp.body());
  }
}
```

### 5.2 SSE 流式调用（简化版）

```java
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

public class StreamExample {
  public static void main(String[] args) throws Exception {
    String body = """
      {
        "prompt":"流式分析项目模块",
        "session_id":"java-stream-001",
        "timeout_ms":600000
      }
      """;

    HttpClient client = HttpClient.newBuilder()
      .connectTimeout(Duration.ofSeconds(10))
      .build();

    HttpRequest req = HttpRequest.newBuilder()
      .uri(URI.create("http://localhost:8080/v1/run/stream"))
      .timeout(Duration.ofSeconds(610))
      .header("Content-Type", "application/json")
      .POST(HttpRequest.BodyPublishers.ofString(body))
      .build();

    HttpResponse<java.io.InputStream> resp =
      client.send(req, HttpResponse.BodyHandlers.ofInputStream());
    if (resp.statusCode() >= 400) {
      throw new RuntimeException("HTTP " + resp.statusCode());
    }

    try (BufferedReader br = new BufferedReader(new InputStreamReader(resp.body()))) {
      String line;
      while ((line = br.readLine()) != null) {
        if (line.startsWith("data: ")) {
          String json = line.substring(6);
          System.out.println(json);
        }
      }
    }
  }
}
```

## 6. 错误码与处理建议

- `400`：请求体错误（字段缺失、JSON 非法）
- `405`：HTTP 方法不对
- `502`：Runtime 执行失败（模型/工具链路报错）
- `500`：服务端内部错误

建议：

1. 对 `502/500` 做有限重试（指数退避）
2. 对 `400` 不重试，直接修正请求
3. 设置客户端超时略大于 `timeout_ms`

## 7. session_id 使用建议（非常重要）

同一个 `session_id` 在服务端会串行执行。  
所以：

1. 无状态调用：每次请求生成唯一 `session_id`
2. 有状态对话：同一用户会话复用 `session_id`
3. 高并发场景不要让所有请求共用同一个 `session_id`

## 8. 生产化建议

1. 前面加网关（鉴权、限流、审计）
2. 将 `examples/03-http` 作为参考，封装成你自己的服务入口
3. 增加请求日志（request_id / session_id / latency / status）
4. 对流式连接设置最大时长和连接数限制
5. 按业务需要隔离模型配置与 `ConfigRoot`

