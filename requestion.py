import requests

# 发送请求
response = requests.post(
    "http://localhost:3000/v1/chat/completions",
    headers={"Authorization": "Bearer sk-cFlmQUgFBMyoPR6z7c70Ad22108640568c64D75eFaBf64B3"},
    json={
        "model": "gemini-1.5-flash",
        # "model": "deepseek-chat",
        "messages": [{"role": "user", "content": "你是什么大模型，介绍一下自己"}]
    }
)

# 打印结果
print(response.json())
