import { useEffect, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../../shared/auth';
import { useToast } from '../../shared/toast';
import { api } from '../../shared/api';

export function InviteRoute() {
  const { token } = useAuth();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { toast } = useToast();
  const joinAttempted = useRef(false);

  const inviteToken = searchParams.get('token');
  const isLoggedIn = !!token;

  useEffect(() => {
    if (!inviteToken) {
      toast('No invitation token provided', 'error');
      navigate('/', { replace: true });
      return;
    }

    if (!isLoggedIn) {
      // 未登录 → 跳转到注册页，带上邀请token
      navigate(`/register?token=${encodeURIComponent(inviteToken)}`, { replace: true });
      return;
    }

    // 已登录 → 直接加入项目
    if (joinAttempted.current) return;
    joinAttempted.current = true;

    const joinProject = async () => {
      try {
        const result = await api.joinProjectWithInvitation(inviteToken);
        if (result.success) {
          toast(`Successfully joined project: ${result.project_name}`, 'success');
          navigate(`/project/${result.project_id}`, { replace: true });
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to join project';
        if (msg.includes('already have access')) {
          toast('You already have access to this project', 'success');
          navigate('/', { replace: true });
          return;
        }
        toast(msg, 'error');
        navigate('/', { replace: true });
      }
    };

    joinProject();
  }, [inviteToken, isLoggedIn, navigate, toast]);

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: '#0a0a0a',
      color: '#a1a1aa',
      fontFamily: 'Inter, system-ui, sans-serif',
    }}>
      {isLoggedIn ? 'Joining project...' : 'Redirecting to registration...'}
    </div>
  );
}
