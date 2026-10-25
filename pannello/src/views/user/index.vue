<template>
  <div>
    <el-card class="list-query" shadow="hover">
      <el-form inline label-width="80px">
        <el-form-item :label="T('Username')">
          <el-input v-model="listQuery.username"></el-input>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handlerQuery">{{ T('Filter') }}</el-button>
          <el-button type="primary" @click="toAdd">{{ T('Add') }}</el-button>
          <el-button @click="toExport">{{ T('Export') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <el-card class="list-body" shadow="hover">
      <el-table :data="listRes.list" v-loading="listRes.loading" border show-overflow-tooltip>
        <el-table-column prop="id" label="ID" align="center" width="70"></el-table-column>
        <el-table-column prop="username" :label="T('Username')" align="center" min-width="120"/>
        <el-table-column prop="email" :label="T('Email')" align="center" min-width="120"/>
        <el-table-column prop="nickname" :label="T('Nickname')" align="center" min-width="120"/>
        <el-table-column :label="T('Group')" align="center" min-width="110">
          <template #default="{row}">
            <span v-if="row.group_id"> <el-tag>{{ listRes.groups?.find(g => g.id === row.group_id)?.name }} </el-tag> </span>
            <span v-else> - </span>
          </template>
        </el-table-column>
        <el-table-column :label="T('Status')" align="center" width="80">
          <template #default="{row}">
            <el-switch v-model="row.status"
                       :active-value="ENABLE_STATUS"
                       :inactive-value="DISABLE_STATUS"
                       @change="changeStatus(row)"
            ></el-switch>
          </template>
        </el-table-column>
        <el-table-column prop="remark" :label="T('Remark')" align="center"/>
        <el-table-column prop="created_at" :label="T('CreatedAt')" align="center" min-width="170"/>
        <el-table-column prop="updated_at" :label="T('UpdatedAt')" align="center" min-width="170"/>
        <el-table-column :label="T('Actions')" align="center" width="180" fixed="right">
          <template #default="{row}">
            <azioni-riga @elimina="remove(row)">
              <el-button link type="primary" @click="toEdit(row)">{{ T('Edit') }}</el-button>
              <template #altro>
                <el-dropdown-item @click="changePass(row)">{{ T('ResetPassword') }}</el-dropdown-item>
                <el-dropdown-item @click="toTag(row)">{{ T('UserTags') }}</el-dropdown-item>
                <el-dropdown-item @click="toAddressBook(row)">{{ T('UserAddressBook') }}</el-dropdown-item>
              </template>
            </azioni-riga>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    <el-card class="list-page" shadow="hover">
      <el-pagination background
                     layout="prev, pager, next, sizes, jumper"
                     :page-sizes="[10,20,50,100]"
                     v-model:page-size="listQuery.page_size"
                     v-model:current-page="listQuery.page"
                     :total="listRes.total">
      </el-pagination>
    </el-card>
  </div>
</template>

<script setup>
  import { useRepositories, useDel, useToEditOrAdd, useChangePwd } from '@/views/user/composables'
  import { T } from '@/utils/i18n'
  import { DISABLE_STATUS, ENABLE_STATUS } from '@/utils/common_options'
  import { update } from '@/api/user'
  import { ElMessageBox, ElMessage } from 'element-plus'
  import { onMounted, watch } from 'vue'
  //列表
  const {
    listRes,
    listQuery,
    handlerQuery,
    getList,
    getGroups,
    toExport,
  } = useRepositories()

  onMounted(getGroups)

  onMounted(getList)

  watch(() => listQuery.page, getList)
  watch(() => listQuery.page_size, handlerQuery)

  const { toEdit, toAdd, toAddressBook, toTag } = useToEditOrAdd()

  const { changePass } = useChangePwd()

  //删除
  const { del } = useDel()
  const remove = async (row) => {
    const res = await del(row.id)
    if (res) {
      getList(listQuery)
    }
  }

  const changeStatus = async (row) => {
    /*const confirm = await ElMessageBox.confirm(T('Confirm?', { param: T('Update') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
    }).catch(_ => false)
    if (!confirm) {
      return false
    }*/
    const res = await update(row).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      getList(listQuery)
    }
  }

</script>

<style scoped>
</style>
