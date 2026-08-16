import { PageLayout } from '@/components/page-layout'

export function PlaceholderPage({
  title,
  description,
}: {
  title: string
  description: string
}) {
  return (
    <PageLayout title={title} description={description}>
      <div className="ff-placeholder">功能开发中，接口就绪后接入</div>
    </PageLayout>
  )
}
