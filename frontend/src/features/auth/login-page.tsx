'use client';

import {useState, type CSSProperties} from 'react';
import {useNavigate, useSearch} from '@tanstack/react-router';
import {VStack, StackItem} from '@astryxdesign/core/Layout';
import {Grid} from '@astryxdesign/core/Grid';
import {Center} from '@astryxdesign/core/Center';
import {Card} from '@astryxdesign/core/Card';
import {Text} from '@astryxdesign/core/Text';
import {Icon} from '@astryxdesign/core/Icon';
import {EmptyState} from '@astryxdesign/core/EmptyState';
import {CheckCircleIcon} from '@heroicons/react/24/outline';
import {TextInput} from '@astryxdesign/core/TextInput';
import {Button} from '@astryxdesign/core/Button';
import {Link} from '@astryxdesign/core/Link';
import {useToast} from '@astryxdesign/core/Toast';
import {login} from '@/apis/auth-api';
import {getErrorMessage} from '@/lib/http';
import {useAuthStore} from '@/stores/auth-store';

const COVER_IMAGE_URL = '/images/pexels-fabriziovelez-14980647.jpg';

const COLUMN_MIN_WIDTH = 280;
const pageStyle: CSSProperties = {
  height: '100dvh',
  width: '100%',
  background: 'linear-gradient(180deg, #eef3f8 0%, #f7fbff 100%)',
  overflow: 'hidden',
};
const coverImage: CSSProperties = {
  width: '100%',
  height: '100%',
  objectFit: 'cover',
  display: 'block',
};

const LOGIN_SPLIT_CSS = `
.login-split-grid {
  container-type: inline-size;
  container-name: login-split;
  min-height: 100dvh;
  width: 100%;
}
.login-split-image {
  width: 100%;
  min-height: 100dvh;
  order: 0;
}
.login-split-form {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100dvh;
  padding: var(--spacing-8);
}
.login-split-panel {
  width: min(560px, 100%);
  padding: var(--spacing-10);
  border-radius: 28px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 24px 80px rgba(15, 23, 42, 0.16);
}
.login-split-logo {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  margin-bottom: var(--spacing-6);
}
.login-split-logo img {
  height: 40px;
  width: auto;
}
.login-split-title {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--spacing-2);
  margin-bottom: var(--spacing-5);
}
@container login-split (max-width: 899px) {
  .login-split-form {
    padding: var(--spacing-6);
  }
  .login-split-panel {
    width: 100%;
    border-radius: 20px;
    box-shadow: none;
    background: rgba(255, 255, 255, 0.98);
  }
}
@container login-split (max-width: 679px) {
  .login-split-image {
    order: -1;
    min-height: 280px;
  }
  .login-split-grid {
    padding: 0;
  }
}
`;

export function LoginPage() {
  const navigate = useNavigate();
  const search = useSearch({from: '/login'});
  const toast = useToast();
  const setUser = useAuthStore((s) => s.setUser);
  const [email, setEmail] = useState('admin');
  const [password, setPassword] = useState('123456');
  const [loginFailed, setLoginFailed] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);

  const handleLogin = async () => {
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
      setIsSuccess(true);
      const redirect = (search as {redirect?: string}).redirect || '/devices';
      await navigate({to: redirect});
    } catch (error) {
      toast({body: getErrorMessage(error, '登录失败'), type: 'error'});
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div style={pageStyle}>
      <style>{LOGIN_SPLIT_CSS}</style>
      <Grid
        columns={{minWidth: COLUMN_MIN_WIDTH, repeat: 'fit'}}
        gap={0}
        align="stretch"
        style={{height: '100%', width: '100%'}}
        className="login-split-grid">
        <div className="login-split-image">
          <Card variant="transparent" padding={0} width="100%" height="100%">
            <img
              style={coverImage}
              src={COVER_IMAGE_URL}
              alt="高科技城市 AI 主题封面"
            />
          </Card>
        </div>
        <div className="login-split-form">
          <div className="login-split-panel">
            <div className="login-split-logo">
              <img src="/frameflow_logo_full.png" alt="FrameFlow 标志" />
            </div>
            <VStack gap={4} style={{height: '100%'}}>
              <div className="login-split-title">
                <Text type="display-1" as="h2">
                  欢迎来到 FrameFlow
                </Text>
                <Text type="body" color="secondary" size="sm">
                  登录你的 FrameFlow 帐户，开启智能巡帧体验
                </Text>
              </div>

              <StackItem size="fill">
                <Center axis="vertical" height="100%">
                  {isSuccess ? (
                    <EmptyState
                      title="登录成功"
                      description="正在跳转到你的工作区..."
                      icon={<Icon icon={CheckCircleIcon} size="lg" />}
                    />
                  ) : (
                    <VStack gap={4} hAlign="stretch" width="100%">
                      <TextInput
                        label="用户名"
                        isLabelHidden
                        type="email"
                        placeholder="请输入邮箱地址"
                        value={email}
                        onChange={setEmail}
                        size="lg"
                      />
                      <VStack gap={1}>
                        <TextInput
                          label="密码"
                          isLabelHidden
                          placeholder="请输入密码"
                          type="password"
                          value={password}
                          onChange={(v: string) => {
                            setPassword(v);
                            setLoginFailed(false);
                          }}
                          size="lg"
                          status={
                            loginFailed
                              ? {
                                  type: 'error',
                                  message: '用户名或密码错误，请重试。',
                                }
                              : undefined
                          }
                        />
                      </VStack>

                      <Button
                        label="登录"
                        variant="primary"
                        size="lg"
                        isLoading={isLoading}
                        onClick={handleLogin}
                      />
                    </VStack>
                  )}
                </Center>
              </StackItem>

              {!isSuccess && (
                <Text type="supporting" color="secondary">
                  如果没有账号，{' '}
                  <Link href="#" type="supporting">
                    立即注册
                  </Link>
                  。
                </Text>
              )}
            </VStack>
          </div>
        </div>
      </Grid>

      <VStack hAlign="center" style={{padding: '0 var(--spacing-8)'}}>
        <Text type="supporting" color="secondary">
          点击继续即表示您同意我们的{' '}
          <Link href="#" type="supporting">
            服务条款
          </Link>{' '}
          和{' '}
          <Link href="#" type="supporting">
            隐私政策
          </Link>
          。
        </Text>
      </VStack>
    </div>
  );
}
