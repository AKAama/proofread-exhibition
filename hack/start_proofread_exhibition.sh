#!/bin/bash

# 用法: ./start_ai_proofread.sh [start|stop|restart|status|logs|pull]

# 配置参数
CONTAINER_NAME="proofread-exhibition"
IMAGE_NAME="testhub.sudytech.cn/webplus/proofread-exhibition"
IMAGE_TAG="1.0.0"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"
HOST_PORT="8701"
CONTAINER_PORT="3000"

# 数据卷挂载配置
HOST_DIR="/opt/proofread-exhibition"
HOST_ETC_DIR="${HOST_DIR}/etc"
HOST_TPL_DIR="${HOST_DIR}/pkg/tpl"
HOST_CONFIG_FILE="${HOST_ETC_DIR}/config.yaml"
CONTAINER_CONFIG_PATH="/app/etc/config.yaml"
HOST_TPL_FILE="${HOST_TPL_DIR}/index.html"
CONTAINER_TPL_PATH="/app/pkg/tpl/index.html"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# 检查容器是否存在
container_exists() {
    docker ps -a --format "table {{.Names}}" | grep -q "^${CONTAINER_NAME}$"
}

# 检查容器是否运行中
container_running() {
    docker ps --format "table {{.Names}}" | grep -q "^${CONTAINER_NAME}$"
}

# 检查镜像是否存在
image_exists() {
    docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "^${FULL_IMAGE_NAME}$"
}

# 检查挂载目录是否存在
check_mount_directories() {
    # 检查 etc 目录
    if [ ! -d "$HOST_ETC_DIR" ]; then
        print_warn "主机挂载目录不存在: $HOST_ETC_DIR"
        read -p "是否创建该目录? (y/n): " -r
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            mkdir -p "$HOST_ETC_DIR"
            # shellcheck disable=SC2181
            if [ $? -eq 0 ]; then
                print_info "目录创建成功: $HOST_ETC_DIR"
            else
                print_error "目录创建失败: $HOST_ETC_DIR"
                return 1
            fi
        else
            print_error "挂载目录不存在，容器启动可能失败"
            return 1
        fi
    else
        print_info "挂载目录检查通过: $HOST_ETC_DIR"
    fi

    # 检查配置文件是否存在
    if [ ! -f "$HOST_CONFIG_FILE" ]; then
        print_warn "配置文件不存在: $HOST_CONFIG_FILE"
        print_warn "请确保配置文件已放置在正确位置"
    else
        print_info "配置文件检查通过: $HOST_CONFIG_FILE"
    fi

    # 检查模板文件目录
    if [ ! -d "$HOST_TPL_DIR" ]; then
        print_warn "主机模板目录不存在: $HOST_TPL_DIR"
        read -p "是否创建该目录? (y/n): " -r
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            mkdir -p "$HOST_TPL_DIR"
            # shellcheck disable=SC2181
            if [ $? -eq 0 ]; then
                print_info "目录创建成功: $HOST_TPL_DIR"
            else
                print_error "目录创建失败: $HOST_TPL_DIR"
                return 1
            fi
        else
            print_error "模板目录不存在，容器启动可能失败"
            return 1
        fi
    else
        print_info "模板目录检查通过: $HOST_TPL_DIR"
    fi

    # 检查模板文件是否存在
    if [ ! -f "$HOST_TPL_FILE" ]; then
        print_warn "模板文件不存在: ${HOST_TPL_FILE}"
        print_warn "请确保模板文件已放置在正确位置"
    else
        print_info "模板文件检查通过: ${HOST_TPL_FILE}"
    fi
}

# 拉取镜像
pull_image() {
    print_info "正在拉取镜像 ${FULL_IMAGE_NAME}..."

    docker pull ${FULL_IMAGE_NAME}

    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
        print_info "镜像拉取成功！"
        return 0
    else
        print_error "镜像拉取失败！"
        print_error "请检查："
        print_error "1. 网络连接是否正常"
        print_error "2. Docker registry 认证是否正确"
        print_error "3. 镜像名称和标签是否正确"
        return 1
    fi
}

# 确保镜像存在
ensure_image() {
    if image_exists; then
        print_info "本地镜像已存在: ${FULL_IMAGE_NAME}"

        # 询问是否要更新镜像
        read -p "是否拉取最新镜像? (y/n): " -r
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            pull_image
            return $?
        fi
        return 0
    else
        print_warn "本地镜像不存在，正在拉取..."
        pull_image
        return $?
    fi
}

# 启动容器
start_container() {
    print_info "准备启动容器 ${CONTAINER_NAME}..."

    # 1. 停止并删除原有容器
    if container_exists; then
        print_info "检测到已有同名容器，正在停止并删除..."
        if container_running; then
            docker stop ${CONTAINER_NAME}
        fi
        docker rm ${CONTAINER_NAME}
        print_info "原有容器已删除"
    fi

    # 2. 确保镜像存在
    ensure_image
    # shellcheck disable=SC2181
    if [ $? -ne 0 ]; then
        print_error "镜像准备失败，无法启动容器"
        return 1
    fi

    # 3. 检查挂载目录
    check_mount_directories
    # shellcheck disable=SC2181
    if [ $? -ne 0 ]; then
        return 1
    fi

    # 4. 启动容器
    print_info "启动新容器..."
    print_debug "挂载配置: ${HOST_CONFIG_FILE} -> ${CONTAINER_CONFIG_PATH}"
    print_debug "挂载配置: ${HOST_TPL_FILE} -> ${CONTAINER_TPL_PATH}"

    docker run -d \
        -p "${HOST_PORT}:${CONTAINER_PORT}" \
        -v "${HOST_CONFIG_FILE}:${CONTAINER_CONFIG_PATH}" \
        -v "${HOST_TPL_FILE}:${CONTAINER_TPL_PATH}" \
        --name "${CONTAINER_NAME}" \
        "${FULL_IMAGE_NAME}"

    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
        print_info "容器启动成功！"
        sleep 2
        print_info "容器日志:"
        docker logs ${CONTAINER_NAME}
    else
        print_error "容器启动失败！"
        print_error "错误日志:"
        docker logs ${CONTAINER_NAME}
        return 1
    fi
}

# 停止容器
stop_container() {
    print_info "停止容器 ${CONTAINER_NAME}..."

    if ! container_running; then
        print_warn "容器 ${CONTAINER_NAME} 未在运行"
        return 0
    fi

    docker stop ${CONTAINER_NAME}

    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
        print_info "容器停止成功！"
    else
        print_error "容器停止失败！"
        return 1
    fi
}

# 重启容器
restart_container() {
    print_info "重启容器 ${CONTAINER_NAME}..."
    stop_container
    sleep 2
    start_container
}

# 查看容器状态
show_status() {
    print_info "容器状态信息："
    echo "----------------------------------------"

    if container_exists; then
        docker ps -a --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Image}}"
        echo "----------------------------------------"

        if container_running; then
            print_info "容器正在运行中"
            echo "访问地址: http://127.0.0.1:${HOST_PORT}"
            echo "配置文件: ${HOST_CONFIG_FILE}"
        else
            print_warn "容器已停止"
        fi
    else
        print_warn "容器 ${CONTAINER_NAME} 不存在"
    fi

    # 显示镜像信息
    echo ""
    print_info "镜像信息："
    if image_exists; then
        docker images --filter "reference=${IMAGE_NAME}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"
    else
        print_warn "本地未找到镜像: ${FULL_IMAGE_NAME}"
    fi
}

# 查看日志
show_logs() {
    if container_exists; then
        print_info "显示容器日志（按 Ctrl+C 退出）："
        docker logs -f ${CONTAINER_NAME}
    else
        print_error "容器 ${CONTAINER_NAME} 不存在"
        return 1
    fi
}

# 删除容器
remove_container() {
    print_info "删除容器 ${CONTAINER_NAME}..."

    if container_running; then
        print_info "先停止运行中的容器..."
        stop_container
    fi

    if container_exists; then
        docker rm ${CONTAINER_NAME}
        # shellcheck disable=SC2181
        if [ $? -eq 0 ]; then
            print_info "容器删除成功！"
        else
            print_error "容器删除失败！"
            return 1
        fi
    else
        print_warn "容器 ${CONTAINER_NAME} 不存在"
    fi
}

# 进入容器
enter_container() {
    if container_running; then
        print_info "进入容器 ${CONTAINER_NAME}..."
        docker exec -it ${CONTAINER_NAME} sh
    else
        print_error "容器 ${CONTAINER_NAME} 未运行"
        return 1
    fi
}

# 更新镜像和重启服务
update_service() {
    print_info "更新服务..."

    # 1. 拉取最新镜像
    pull_image
    # shellcheck disable=SC2181
    if [ $? -ne 0 ]; then
        print_error "镜像更新失败"
        return 1
    fi

    # 2. 重启容器
    restart_container

    # shellcheck disable=SC2181
    if [ $? -eq 0 ]; then
        print_info "服务更新完成！"
    else
        print_error "服务更新失败！"
        return 1
    fi
}

# 清理旧镜像
cleanup_images() {
    print_info "清理旧镜像..."

    # 显示所有相关镜像
    print_info "当前相关镜像："
    docker images --filter "reference=${IMAGE_NAME}" --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}"

    # 清理悬空镜像
    print_info "清理悬空镜像..."
    docker image prune -f

    print_info "镜像清理完成"
}

# 调试功能 - 检查挂载
debug_mount() {
    print_info "调试挂载信息..."
    print_debug "主机配置文件: ${HOST_CONFIG_FILE}"
    print_debug "容器配置文件: ${CONTAINER_CONFIG_PATH}"
    print_debug "主机模板文件: ${HOST_TPL_FILE}"
    print_debug "容器模板文件: ${CONTAINER_TPL_PATH}"

    # 检查主机 etc 目录
    print_info "主机 etc 目录内容:"
    ls -la ${HOST_ETC_DIR}/ 2>/dev/null || print_error "主机 etc 目录不存在或无权限访问"

    # 检查主机模板目录
    print_info "主机模板目录内容:"
    ls -la ${HOST_TPL_DIR}/ 2>/dev/null || print_error "主机模板目录不存在或无权限访问"

    if container_running; then
        print_info "容器内 etc 目录内容:"
        docker exec ${CONTAINER_NAME} ls -la /app/etc/ 2>/dev/null || print_error "容器内 etc 目录不存在"

        print_info "容器内模板目录内容:"
        docker exec ${CONTAINER_NAME} ls -la /app/pkg/tpl/ 2>/dev/null || print_error "容器内模板目录不存在"

        print_info "容器内工作目录:"
        docker exec ${CONTAINER_NAME} pwd

        print_info "容器挂载信息:"
        docker inspect ${CONTAINER_NAME} | grep -A 10 "Mounts"
    else
        print_warn "容器未运行，无法检查内部挂载"
    fi
}

# 显示帮助信息
show_help() {
    echo "Docker容器管理脚本 - WebQA API 服务"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "可用命令:"
    echo "  start    启动容器"
    echo "  stop     停止容器"
    echo "  restart  重启容器"
    echo "  status   查看容器状态"
    echo "  logs     查看容器日志"
    echo "  remove   删除容器"
    echo "  enter    进入容器"
    echo "  pull     拉取最新镜像"
    echo "  update   更新服务(拉取镜像+重启)"
    echo "  cleanup  清理旧镜像"
    echo "  debug    调试挂载问题"
    echo "  help     显示此帮助信息"
    echo ""
    echo "如果不指定命令，默认执行 start"
    echo ""
    echo "配置信息:"
    echo "  容器名称: ${CONTAINER_NAME}"
    echo "  镜像名称: ${FULL_IMAGE_NAME}"
    echo "  端口映射: ${HOST_PORT}:${CONTAINER_PORT}"
    echo "  主机配置文件: ${HOST_CONFIG_FILE}"
    echo "  容器配置文件: ${CONTAINER_CONFIG_PATH}"
    echo "  主机模板文件: ${HOST_TPL_FILE}"
    echo "  容器模板文件: ${CONTAINER_TPL_PATH}"
    echo "  挂载目录: ${HOST_ETC_DIR}, ${HOST_TPL_DIR}"
    echo ""
    echo "注意事项:"
    echo "  1. 首次运行会自动拉取镜像"
    echo "  2. 挂载目录不存在时会提示创建"
    echo "  3. 使用 update 命令可以一键更新服务"
}

# 主逻辑
case "${1:-start}" in
    start)
        start_container
        ;;
    stop)
        stop_container
        ;;
    restart)
        restart_container
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    remove|rm)
        remove_container
        ;;
    enter|exec)
        enter_container
        ;;
    pull)
        pull_image
        ;;
    update)
        update_service
        ;;
    cleanup)
        cleanup_images
        ;;
    debug)
        debug_mount
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "未知命令: $1"
        echo ""
        show_help
        exit 1
        ;;
esac

