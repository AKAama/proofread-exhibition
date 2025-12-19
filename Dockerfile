FROM hub.sudytech.cn/library/alpine:3.16
ARG TARGETARCH
ARG APP_NAME
ENV APP_NAME=${APP_NAME}
LABEL maintainer="fanxun <xunfan@sudytech.com>"
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add -U tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

# 创建应用目录
RUN mkdir -p /app/etc

# 复制二进制文件
COPY out/${APP_NAME}-linux-${TARGETARCH} /app/${APP_NAME}
RUN chmod +x /app/${APP_NAME}

# 复制配置文件
COPY etc/config.yaml /app/etc/config.yaml

# 复制模板与静态资源
COPY tpl /app/tpl

WORKDIR /app
ENTRYPOINT ["sh", "-c", "/app/${APP_NAME} --config /app/etc/config.yaml"]