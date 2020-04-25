<template>
    <tr v-bind:class="{'bg-isUser': team.is_user}">
        <td v-if="pos < 4" class="text-center">
            <span class="icon" v-bind:class="{ 'has-text-warning': pos === 1, 'is-silver': pos === 2, 'is-bronze': pos === 3 }">
              <i class="fas fa-trophy"></i>
            </span>
        </td>
        <td v-else class="text-center">{{ pos }}</td>
        <td><strong>{{ team.name }}</strong></td>
        <td class="text-center">{{ team.tpoints }}</td>
        <challenge-cell v-for="comp in team.completions" v-bind:key="comp" :completed="comp != null"></challenge-cell>
    </tr>
</template>

<script>
    import ChallengeCell from './ChallengeCell.vue'

    export default {
        name: 'team-row',
        props: {
            team: Object,
            pos: Number,
        },
        components: {
            ChallengeCell,
        }
    }
</script>

<style>
    .bg-isUser {
        background-color: rgb(195, 203, 228)!important;
    }
</style>