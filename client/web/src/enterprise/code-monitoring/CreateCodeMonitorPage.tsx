import * as H from 'history'
import VideoInputAntennaIcon from 'mdi-react/VideoInputAntennaIcon'
import React, { useCallback, useMemo, useState } from 'react'
import { AuthenticatedUser } from '../../auth'
import { BreadcrumbSetters, BreadcrumbsProps } from '../../components/Breadcrumbs'
import { PageHeader } from '../../components/PageHeader'
import { PageTitle } from '../../components/PageTitle'
import { createCodeMonitor } from './backend'
import { MonitorEmailPriority } from '../../../../shared/src/graphql/schema'
import { Observable } from 'rxjs'
import { CodeMonitorForm } from './components/CodeMonitorForm'
import { CodeMonitorFields } from '../../graphql-operations'

export interface CreateCodeMonitorPageProps extends BreadcrumbsProps, BreadcrumbSetters {
    location: H.Location
    authenticatedUser: AuthenticatedUser
}

export const CreateCodeMonitorPage: React.FunctionComponent<CreateCodeMonitorPageProps> = props => {
    props.useBreadcrumb(
        useMemo(
            () => ({
                key: 'Create Code Monitor',
                element: <>Create new code monitor</>,
            }),
            []
        )
    )

    const createMonitorRequest = useCallback(
        (codeMonitor: CodeMonitorFields): Observable<Partial<CodeMonitorFields>> =>
            createCodeMonitor({
                monitor: {
                    namespace: props.authenticatedUser.id,
                    description: codeMonitor.description,
                    enabled: codeMonitor.enabled,
                },
                trigger: { query: codeMonitor.trigger.query },

                actions: codeMonitor.actions.nodes.map(action => ({
                    email: {
                        enabled: action.enabled,
                        priority: MonitorEmailPriority.NORMAL,
                        recipients: [props.authenticatedUser.id],
                        header: '',
                    },
                })),
            }),
        [props.authenticatedUser.id]
    )

    return (
        <div className="container mt-3 web-content">
            <PageTitle title="Create new code monitor" />
            <PageHeader title="Create new code monitor" icon={VideoInputAntennaIcon} />
            Code monitors watch your code for specific triggers and run actions in response.{' '}
            <a href="" target="_blank" rel="noopener">
                {/* TODO: populate link */}
                Learn more
            </a>
            <CodeMonitorForm {...props} onSubmit={createMonitorRequest} />
        </div>
    )
}
