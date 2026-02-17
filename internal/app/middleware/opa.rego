package middleware

import future.keywords.contains
import future.keywords.if
import future.keywords.in

# 统一的 RBAC + 资源级别权限检查规则
# 替代原有的 GRBAC (auth_rbac.yml) 和 OPA 两层检查

# ============================================================================
# 第一部分：路由级别权限检查 (从 auth_rbac.yml 转换)
# ============================================================================

# 匿名用户允许访问的路由（精确匹配或模式匹配）
is_public_route if {
    # GET /api/system/options/core.site
    input.method == "GET"
    input.path == "/api/system/options/core.site"
} else if {
    # POST /api/tokens
    input.method == "POST"
    input.path == "/api/tokens"
} else if {
    # DELETE /api/tokens
    input.method == "DELETE"
    input.path == "/api/tokens"
} else if {
    # POST /api/users (注册)
    input.method == "POST"
    input.path == "/api/users"
} else if {
    # PATCH /api/users (修改用户 - 任何人都可以修改自己的信息)
    input.method == "PATCH"
    input.path == "/api/users"
} else if {
    # GET /api/shares/** (查看分享)
    input.method == "GET"
    startswith(input.path, "/api/shares/")
} else if {
    # POST /api/shares/*/token (提取分享)
    input.method == "POST"
    regex.match(`^/api/shares/.+/token$`, input.path)
} else if {
    # GET /api/matters/*/link (下载文件)
    input.method == "GET"
    regex.match(`^/api/matters/.+/link$`, input.path)
}

# 仅管理员可以访问的路由
requires_admin if {
    # POST /api/storages
    input.method == "POST"
    input.path == "/api/storages"
} else if {
    # PUT/PATCH/DELETE /api/storages/**
    input.method in ["PUT", "PATCH", "DELETE"]
    startswith(input.path, "/api/storages/")
} else if {
    # GET /api/users (仅管理员可查看用户列表)
    input.method == "GET"
    input.path == "/api/users"
} else if {
    # PUT/DELETE /api/users/**
    input.method in ["PUT", "DELETE"]
    startswith(input.path, "/api/users/")
} else if {
    # PUT /api/system/options/** (仅管理员可修改配置)
    input.method == "PUT"
    startswith(input.path, "/api/system/options/")
} else if {
    # GET /api/system/options/core.email (仅管理员可查看邮件配置)
    input.method == "GET"
    input.path == "/api/system/options/core.email"
}

# 检查是否是匿名用户
is_anonymous if {
    input.roles[_] == "guest"
}

# 检查是否是管理员
is_admin if {
    input.roles[_] == "admin"
}

# ============================================================================
# 第二部分：权限决策逻辑
# ============================================================================

# 匿名用户访问公开路由 → 允许
allow if {
    is_anonymous
    is_public_route
}

# 已登录用户访问公开路由 → 允许
allow if {
    not is_anonymous
    is_public_route
}

# Note: Anonymous users accessing non-public routes and logged-in users without admin
# accessing admin routes are implicitly denied (not covered by any allow rule)

# 已登录用户访问非管理路由 → 需要进行资源级别检查
allow if {
    not is_anonymous
    not requires_admin
    not is_public_route
    valid_resource_access
}

# 管理员访问任何路由 → 允许
allow if {
    is_admin
}

# ============================================================================
# 第三部分：资源级别权限检查
# ============================================================================

# 有效的资源访问权限
valid_resource_access if {
    input.method == "GET"
    can_read_resource
} else if {
    input.method in ["PATCH", "PUT"]
    can_modify_resource
} else if {
    input.method == "DELETE"
    can_delete_resource
} else if {
    input.method == "POST"
    can_create_resource
}

# 读权限：如果是共享文件或自己的资源
can_read_resource if {
    # 没有资源的 POST 请求（如创建分享）
    input.resource == null
} else if {
    # 如果启用了 share_all_files，允许读取所有资源
    input.config.share_all_files
} else if {
    # 否则只能读取自己的资源
    input.resource.data.uid == input.uid
}

# 修改权限：只能修改自己的资源
can_modify_resource if {
    input.resource.data.uid == input.uid
}

# 删除权限：只能删除自己的资源
can_delete_resource if {
    input.resource.data.uid == input.uid
}

# 创建权限：通常允许（由业务逻辑检查）
can_create_resource if {
    input.uid > 0
}