db.user.insertOne({
    // 1. 登录核心字段
    username: "13800138000",    // 登录账号（固定用手机号）
    password: "123456",        // 密码（项目上线必须加密）

    // 2. 用户基础信息
    nickname: "张三",          // 昵称
    phone: "13800138000",      // 手机号（最重要，用于取件码、通知）
    avatar: "",                // 头像

    // 3. 账号状态
    status: 1,                // 1=正常 0=禁用
    role: "user",             // 固定：user=普通用户

    // 5. 系统字段（所有项目通用）
    createTime: new Date(),    // 创建时间
    updateTime: new Date(),    // 更新时间
    isDeleted: 0,              // 0=未删除 1=已删除
    deletedAt: null           // 删除时间，未删除时为 null，删除时设为 new Date()
})