import { Observable, ReplaySubject } from 'rxjs'
import { filter, mergeMap, take, tap } from 'rxjs/operators'

import { gql } from '@sourcegraph/shared/src/graphql/graphql'
import { createAggregateError } from '@sourcegraph/shared/src/util/errors'

import { authRequired } from '../auth'
import { requestGraphQL } from '../backend/graphql'
import { SiteFlagsResult, SiteFlagsVariables } from '../graphql-operations'

import { SiteFlags } from '.'

/**
 * The latest state of the site flags.
 */
export const siteFlags = new ReplaySubject<SiteFlags>(1)

/**
 * refreshSiteFlags refreshes the site flags. The result is available from
 * the siteFlags const.
 */
export function refreshSiteFlags(): Observable<never> {
    return authRequired.pipe(
        take(1),
        filter(authRequired => !authRequired),
        mergeMap(() =>
            requestGraphQL<SiteFlagsResult, SiteFlagsVariables>(gql`
                query SiteFlags {
                    site {
                        ...SiteFlagFields
                    }
                }

                fragment SiteFlagFields on Site {
                    needsRepositoryConfiguration
                    freeUsersExceeded
                    alerts {
                        type
                        message
                        isDismissibleWithKey
                    }
                    sendsEmailVerificationEmails
                    productSubscription {
                        license {
                            expiresAt
                        }
                        noLicenseWarningUserCount
                    }
                }
            `)
        ),
        tap(({ data, errors }) => {
            if (!data || !data.site) {
                throw createAggregateError(errors)
            }
            siteFlags.next(data.site)
        }),
        mergeMap(() => [])
    )
}

refreshSiteFlags()
    .toPromise()
    .then(
        () => undefined,
        error => console.error(error)
    )
