import requests

# 发送请求
response = requests.post(
    "https://one-api-test-171420155991.asia-northeast1.run.app/v1/chat/completions",
    headers={"Authorization": "Bearer sk-IREADznpqw6FioxV41A54dF3F7504dF3Ba7b02Cc6b0844C7"},
    json={
        "model": "gemini-1.5-flash",
        # "model": "deepseek-chat",
        "messages": [{"role": "user", "content": "你是什么大模型，介绍一下自己"}]
    }
)

# 打印结果
print(response.json())
