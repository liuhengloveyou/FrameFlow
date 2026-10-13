'use client';

import { useState, type FormEvent } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import {VStack} from '@astryxdesign/core/Layout';
import {Text} from '@astryxdesign/core/Text';
import {CheckboxInput} from '@astryxdesign/core/CheckboxInput';
import {TextInput} from '@astryxdesign/core/TextInput';
import {Button} from '@astryxdesign/core/Button';
import {useToast} from '@astryxdesign/core/Toast';
import {EyeIcon, EyeSlashIcon, CheckCircleIcon} from '@heroicons/react/24/outline';
import {login} from '@/apis/auth-api';
import {getErrorMessage} from '@/lib/http';
import {useAuthStore} from '@/stores/auth-store';

const SELLING_POINTS = [
  'AI 自动识别，异常秒级上报',
  '告警联动工作流，处置有据可查',
  '本地部署，数据不出园区',
];

export function LoginPage() {
  const navigate = useNavigate();
  const search = useSearch({from: '/login'});
  const toast = useToast();
  const setUser = useAuthStore((s) => s.setUser);
  const [email, setEmail] = useState('admin');
  const [password, setPassword] = useState('123456');
  const [remember, setRemember] = useState(true);
  const [showPassword, setShowPassword] = useState(false);
  const [loginFailed, setLoginFailed] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  const handleLogin = async (e: FormEvent) => {
    e.preventDefault();
    if (!email.trim() || !password) {
      toast({body: '请输入用户名和密码', type: 'error'});
      setLoginFailed(true);
      return;
    }
    setLoginFailed(false);
    setIsLoading(true);
    try {
      const res = await login(email, password);
      setUser(res.user);
      toast({body: '登录成功', type: 'info'});
      const redirect = (search as {redirect?: string}).redirect || '/preview';
      await navigate({to: redirect});
    } catch (error) {
      toast({body: getErrorMessage(error, '登录失败'), type: 'error'});
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="ff-login">
      {/* 左：监控墙 hero */}
      <section className="ff-login-hero">
        <img
          className="ff-login-hero-img"
          src="/images/login-hero.png"
          alt="视频监控墙"
        />
        <div className="ff-login-hero-body">
          <span className="ff-login-brand">
            <img src="/frameflow_logo_sidenav.png" alt="巡帧 FrameFlow" />
          </span>
          <h1 className="ff-login-slogan">
            让每一帧画面
            <br />
            都被认真巡检
          </h1>
          <p className="ff-login-sub">视频智能巡检平台 · AI 识别 · 告警联动</p>
          <ul className="ff-login-points">
            {SELLING_POINTS.map((p) => (
              <li key={p} className="ff-login-point">
                <CheckCircleIcon aria-hidden />
                {p}
              </li>
            ))}
          </ul>
        </div>
      </section>

      {/* 右：登录卡 */}
      <section className="ff-login-panel">
        <form className="ff-login-card" onSubmit={handleLogin}>
          <div className="ff-login-card-head">
            <h2 className="ff-login-card-title">欢迎回来</h2>
            <p className="ff-login-card-desc">登录后开始巡检你的设备</p>
          </div>

          <VStack gap={4}>
            <TextInput
              label="账号"
              isLabelHidden
              placeholder="手机号 / 用户名"
              value={email}
              onChange={setEmail}
              size="lg"
            />
            <div className="ff-login-password">
              <TextInput
                label="密码"
                isLabelHidden
                placeholder="请输入密码"
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(v: string) => {
                  setPassword(v);
                  setLoginFailed(false);
                }}
                size="lg"
                status={
                  loginFailed
                    ? {type: 'error', message: '用户名或密码错误，请重试。'}
                    : undefined
                }
              />
              <button
                type="button"
                className="ff-login-eye"
                aria-label={showPassword ? '隐藏密码' : '显示密码'}
                onClick={() => setShowPassword((v) => !v)}
              >
                {showPassword ? <EyeSlashIcon aria-hidden /> : <EyeIcon aria-hidden />}
              </button>
            </div>

            <div className="ff-login-row">
              <CheckboxInput
                label="记住我"
                value={remember}
                onChange={setRemember}
              />
              <button type="button" className="ff-login-forget">
                忘记密码?
              </button>
            </div>

            <Button
              label="登录"
              variant="primary"
              size="lg"
              isLoading={isLoading}
              clickAction={handleLogin}
            />

            <Button
              label="使用验证码登录"
              variant="secondary"
              size="lg"
              onClick={() =>
                toast({body: '验证码登录暂未开放，请使用账号密码登录', type: 'info'})
              }
            />
          </VStack>

          <Text type="supporting" color="secondary">
            <span className="ff-login-foot">登录遇到问题？联系管理员或查看帮助</span>
          </Text>
        </form>
      </section>

    </div>
  );
}
